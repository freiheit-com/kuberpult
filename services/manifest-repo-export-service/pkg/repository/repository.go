/*This file is part of kuberpult.

Kuberpult is free software: you can redistribute it and/or modify
it under the terms of the Expat(MIT) License as published by
the Free Software Foundation.

Kuberpult is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
MIT License for more details.

You should have received a copy of the MIT License
along with kuberpult. If not, see <https://directory.fsf.org/wiki/License:Expat>.

Copyright freiheit.com*/

package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/freiheit-com/kuberpult/pkg/argocd"
	"github.com/freiheit-com/kuberpult/pkg/config"
	"github.com/freiheit-com/kuberpult/pkg/db"
	"github.com/freiheit-com/kuberpult/pkg/mapper"
	time2 "github.com/freiheit-com/kuberpult/pkg/time"

	"google.golang.org/protobuf/types/known/timestamppb"

	backoff "github.com/cenkalti/backoff/v4"
	api "github.com/freiheit-com/kuberpult/pkg/api/v1"
	"github.com/freiheit-com/kuberpult/pkg/auth"
	"github.com/freiheit-com/kuberpult/services/manifest-repo-export-service/pkg/fs"
	"github.com/freiheit-com/kuberpult/services/manifest-repo-export-service/pkg/sqlitestore"
	"go.uber.org/zap"
	"gopkg.in/DataDog/dd-trace-go.v1/ddtrace/tracer"

	"github.com/freiheit-com/kuberpult/pkg/logger"
	billy "github.com/go-git/go-billy/v5"
	"github.com/go-git/go-billy/v5/util"
	git "github.com/libgit2/git2go/v34"
)

// A Repository provides a multiple reader / single writer access to a git repository.
type Repository interface {
	Apply(ctx context.Context, tx *sql.Tx, transformers ...Transformer) error
	Push(ctx context.Context, pushAction func() error) error
	ApplyTransformersInternal(ctx context.Context, transaction *sql.Tx, transformer Transformer) ([]string, *State, []*TransformerResult, *TransformerBatchApplyError)
	State() *State
	StateAt(oid *git.Oid) (*State, error)
	FetchAndReset(ctx context.Context) error
	PushRepo(ctx context.Context) error
	GetHeadCommit(ctx context.Context) (*git.Commit, error)
}

type TransformerBatchApplyError struct {
	TransformerError error // the error that caused the batch to fail. nil if no error happened
	Index            int   // the index of the transformer that caused the batch to fail or -1 if the error happened outside one specific transformer
}

func (err *TransformerBatchApplyError) Error() string {
	if err == nil {
		return ""
	}
	if err.Index < 0 {
		return fmt.Sprintf("error not specific to one transformer of this batch: %s", err.TransformerError.Error())
	}
	return fmt.Sprintf("error at index %d of transformer batch: %s", err.Index, err.TransformerError.Error())
}

func (err *TransformerBatchApplyError) Is(target error) bool {
	tgt, ok := target.(*TransformerBatchApplyError)
	if !ok {
		return false
	}
	if err == nil {
		return target == nil
	}
	if target == nil {
		return false
	}
	// now both target and err are guaranteed to be non-nil
	if err.Index != tgt.Index {
		return false
	}
	return errors.Is(err.TransformerError, tgt.TransformerError)
}

func defaultBackOffProvider() backoff.BackOff {
	eb := backoff.NewExponentialBackOff()
	eb.MaxElapsedTime = 7 * time.Second
	return backoff.WithMaxRetries(eb, 6)
}

type repository struct {
	config       *RepositoryConfig
	credentials  *credentialsStore
	certificates *certificateStore

	repository *git.Repository

	backOffProvider func() backoff.BackOff

	DB *db.DBHandler
}

type RepositoryConfig struct {
	// Mandatory Config
	// the URL used for git checkout, (ssh protocol)
	URL  string
	Path string
	// Optional Config
	Credentials    Credentials
	Certificates   Certificates
	CommitterEmail string
	CommitterName  string
	// default branch is master
	Branch string
	// network timeout
	NetworkTimeout time.Duration

	ArgoCdGenerateFiles bool
	ReleaseVersionLimit uint
	DBHandler           *db.DBHandler
}

func openOrCreate(path string) (*git.Repository, error) {
	repo2, err := git.OpenRepositoryExtended(path, git.RepositoryOpenNoSearch, path)
	if err != nil {
		var gerr *git.GitError
		if errors.As(err, &gerr) {
			if gerr.Code == git.ErrorCodeNotFound {
				err = os.MkdirAll(path, 0777)
				if err != nil {
					return nil, err
				}
				repo2, err = git.InitRepository(path, true)
				if err != nil {
					return nil, err
				}
			} else {
				return nil, err
			}
		} else {
			return nil, err
		}
	}
	sqlitePath := filepath.Join(path, "odb.sqlite")
	be, err := sqlitestore.NewOdbBackend(sqlitePath)
	if err != nil {
		return nil, fmt.Errorf("creating odb backend: %w", err)
	}
	odb, err := repo2.Odb()
	if err != nil {
		return nil, fmt.Errorf("gettting odb: %w", err)
	}
	// Prioriority 99 ensures that libgit prefers this backend for writing over its buildin backends.
	err = odb.AddBackend(be, 99)
	if err != nil {
		return nil, fmt.Errorf("setting odb backend: %w", err)
	}
	return repo2, err
}

func New(ctx context.Context, cfg RepositoryConfig) (Repository, error) {
	logger := logger.FromContext(ctx)

	if cfg.Branch == "" {
		cfg.Branch = "master"
	}
	if cfg.CommitterEmail == "" {
		cfg.CommitterEmail = "kuberpult@example.com"
	}
	if cfg.CommitterName == "" {
		cfg.CommitterName = "kuberpult"
	}
	if cfg.NetworkTimeout == 0 {
		cfg.NetworkTimeout = time.Minute
	}
	if cfg.ReleaseVersionLimit == 0 {
		cfg.ReleaseVersionLimit = keptVersionsOnCleanup
	}
	var credentials *credentialsStore
	var certificates *certificateStore
	var err error
	if strings.HasPrefix(cfg.URL, "./") || strings.HasPrefix(cfg.URL, "/") {
		logger.Debug("git url indicates a local directory. Ignoring credentials and certificates.")
	} else {
		credentials, err = cfg.Credentials.load()
		if err != nil {
			return nil, err
		}
		certificates, err = cfg.Certificates.load()
		if err != nil {
			return nil, err
		}
	}

	if repo2, err := openOrCreate(cfg.Path); err != nil {
		return nil, err
	} else {
		// configure remotes
		if remote, err := repo2.Remotes.CreateAnonymous(cfg.URL); err != nil {
			return nil, err
		} else {

			result := &repository{
				config:          &cfg,
				credentials:     credentials,
				certificates:    certificates,
				repository:      repo2,
				backOffProvider: defaultBackOffProvider,
				DB:              cfg.DBHandler,
			}
			fetchSpec := fmt.Sprintf("+refs/heads/%s:refs/remotes/origin/%s", cfg.Branch, cfg.Branch)
			//exhaustruct:ignore
			RemoteCallbacks := git.RemoteCallbacks{
				UpdateTipsCallback: func(refname string, a *git.Oid, b *git.Oid) error {
					logger.Debug("git.fetched",
						zap.String("refname", refname),
						zap.String("revision.new", b.String()),
					)
					return nil
				},
				CredentialsCallback:      credentials.CredentialsCallback(ctx),
				CertificateCheckCallback: certificates.CertificateCheckCallback(ctx),
			}
			fetchOptions := git.FetchOptions{
				Prune:           git.FetchPruneUnspecified,
				UpdateFetchhead: false,
				DownloadTags:    git.DownloadTagsUnspecified,
				Headers:         nil,
				ProxyOptions: git.ProxyOptions{
					Type: git.ProxyTypeNone,
					Url:  "",
				},
				RemoteCallbacks: RemoteCallbacks,
			}
			err := remote.Fetch([]string{fetchSpec}, &fetchOptions, "fetching")
			if err != nil {
				return nil, err
			}
			var rev *git.Oid
			if remoteRef, err := repo2.References.Lookup(fmt.Sprintf("refs/remotes/origin/%s", cfg.Branch)); err != nil {
				var gerr *git.GitError
				if errors.As(err, &gerr) && gerr.Code == git.ErrorCodeNotFound {
					// not found
					// nothing to do
				} else {
					return nil, err
				}
			} else {
				rev = remoteRef.Target()
				if _, err := repo2.References.Create(fmt.Sprintf("refs/heads/%s", cfg.Branch), rev, true, "reset branch"); err != nil {
					return nil, err
				}
			}

			// check that we can build the current state
			state, err := result.StateAt(nil)
			if err != nil {
				return nil, err
			}

			// Check configuration for errors and abort early if any:
			_, err = state.GetEnvironmentConfigsAndValidate(ctx)
			if err != nil {
				return nil, err
			}

			return result, nil
		}
	}
}

func (r *repository) applyTransformerBatches(ctx context.Context, transformer Transformer, allowFetchAndReset bool, transaction *sql.Tx) (error, *TransformerResult) {
	span, ctx := tracer.StartSpanFromContext(ctx, "applyTransformerBatches")
	defer span.Finish()
	span.SetTag("allowFetchAndReset", allowFetchAndReset)
	//exhaustruct:ignore
	var changes = &TransformerResult{}
	subChanges, applyErr := r.ApplyTransformer(ctx, transaction, transformer)
	changes.Combine(subChanges)
	if applyErr != nil {
		if errors.Is(applyErr.TransformerError, InvalidJson) && allowFetchAndReset {
			// Invalid state. fetch and reset and redo
			err := r.FetchAndReset(ctx)
			if err != nil {
				return err, nil
			}
			return r.applyTransformerBatches(ctx, transformer, false, transaction)
		} else {
			return applyErr, nil
		}
	}
	return nil, changes
}

func (r *repository) useRemote(callback func(*git.Remote) error) error {
	remote, err := r.repository.Remotes.CreateAnonymous(r.config.URL)
	if err != nil {
		return fmt.Errorf("opening remote %q: %w", r.config.URL, err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), r.config.NetworkTimeout)
	defer cancel()
	errCh := make(chan error, 1)
	go func() {
		// Usually we call `defer` right after resource allocation (`CreateAnonymous`).
		// The issue with that is that the `callback` requires the remote, and cannot be cancelled properly.
		// So `callback` may run longer than `useRemote`, and if at that point `Disconnect` was already called, we get a `panic`.
		defer remote.Disconnect()
		errCh <- callback(remote)
	}()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case err := <-errCh:
		return err
	}
}

// It returns always nil
// success is set to true if the push was successful
func defaultPushUpdate(branch string, success *bool) git.PushUpdateReferenceCallback {
	return func(refName string, status string) error {
		var expectedRefName = fmt.Sprintf("refs/heads/%s", branch)
		// if we were successful the status is empty and the ref contains our branch:
		*success = refName == expectedRefName && status == ""
		return nil
	}
}

type PushActionFunc func() error
type PushActionCallbackFunc func(git.PushOptions, *repository) PushActionFunc

// DefaultPushActionCallback is public for testing reasons only.
func DefaultPushActionCallback(pushOptions git.PushOptions, r *repository) PushActionFunc {
	return func() error {
		return r.useRemote(func(remote *git.Remote) error {
			return remote.Push([]string{fmt.Sprintf("refs/heads/%s:refs/heads/%s", r.config.Branch, r.config.Branch)}, &pushOptions)
		})
	}
}

type PushUpdateFunc func(string, *bool) git.PushUpdateReferenceCallback

func (r *repository) ProcessQueueOnce(ctx context.Context, t Transformer, tx *sql.Tx) error {
	span, ctx := tracer.StartSpanFromContext(ctx, "ProcessQueueOnce")
	defer span.Finish()

	log := logger.FromContext(ctx).Sugar()

	// Apply the items
	apply := func() (error, *TransformerResult) {
		err, changes := r.applyTransformerBatches(ctx, t, true, tx)
		if err != nil {
			log.Warnf("rolling back transaction because of %v", err)
			return err, nil
		}
		return nil, changes
	}

	err, _ := apply()
	if err != nil {
		return fmt.Errorf("first apply failed, aborting: %v", err)
	}
	return nil
}

func (r *repository) PushRepo(ctx context.Context) error {
	var pushSuccess = true
	//exhaustruct:ignore
	RemoteCallbacks := git.RemoteCallbacks{
		CredentialsCallback:         r.credentials.CredentialsCallback(ctx),
		CertificateCheckCallback:    r.certificates.CertificateCheckCallback(ctx),
		PushUpdateReferenceCallback: defaultPushUpdate(r.config.Branch, &pushSuccess),
	}
	pushOptions := git.PushOptions{
		PbParallelism: 0,
		Headers:       nil,
		ProxyOptions: git.ProxyOptions{
			Type: git.ProxyTypeNone,
			Url:  "",
		},
		RemoteCallbacks: RemoteCallbacks,
	}

	// Try pushing once
	err := r.Push(ctx, DefaultPushActionCallback(pushOptions, r))
	if err != nil {
		gerr, ok := err.(*git.GitError)
		// If it doesn't work because the branch diverged, try reset and apply again.
		if ok && gerr.Code == git.ErrorCodeNonFastForward {
			return fmt.Errorf("fastforward error: %w", gerr)
		} else if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
			return fmt.Errorf("context error: %w", err)
		} else {
			logger.FromContext(ctx).Error(fmt.Sprintf("error while pushing: %s", err))
			return fmt.Errorf("could not push to manifest repository '%s' on branch '%s' - this indicates that the ssh key does not have write access", r.config.URL, r.config.Branch)
		}
	} else {
		if !pushSuccess {
			return fmt.Errorf("failed to push - this indicates that branch protection is enabled in '%s' on branch '%s'", r.config.URL, r.config.Branch)
		}
	}
	return nil
}

func (r *repository) GetHeadCommit(ctx context.Context) (*git.Commit, error) {
	msg := ""
	ite, err := r.repository.NewBranchIterator(git.BranchAll)
	if err != nil {
		return nil, fmt.Errorf("Error looping through branches: %v", err)
	} else {
		msg += "Branches Info\n"
		for {
			branch, branchType, error := ite.Next()
			if error != nil {
				break
			}
			name, errorName := branch.Name()
			if errorName == nil {
				msg = msg + fmt.Sprintf("Branch: %v\n", name)
			} else {
				msg = msg + fmt.Sprintf("Failed to get a branch name.\n")
			}

			msg += fmt.Sprintf("\tBranchType: %v\n\tBranch Points to: %v\n", branchType, branch.Reference.Target())
		}
	}

	ref, err := r.repository.Head()
	name, err := ref.Branch().Name()
	if err != nil {
		msg += fmt.Sprintf("Failed to get branch name. %s", err.Error())
	} else {
		msg += fmt.Sprintf("Current Branch: %s\n", name)
	}
	msg += fmt.Sprintf("Head target: %v\n", ref.Target())
	if err != nil {
		return nil, fmt.Errorf("Error fetching HEAD: %v", err)
	}
	commit, err := r.repository.LookupCommit(ref.Target())
	if err != nil {
		return nil, fmt.Errorf("Error transalting into commit: %v", err)
	}
	msg += fmt.Sprintf("Commit id:   %v\n", commit.Id().String())
	logger.FromContext(ctx).Warn(msg)
	return commit, nil

}

func (r *repository) ApplyTransformersInternal(ctx context.Context, transaction *sql.Tx, transformer Transformer) ([]string, *State, []*TransformerResult, *TransformerBatchApplyError) {
	span, ctx := tracer.StartSpanFromContext(ctx, "ApplyTransformersInternal")
	defer span.Finish()
	if state, err := r.StateAt(nil); err != nil {
		return nil, nil, nil, &TransformerBatchApplyError{TransformerError: fmt.Errorf("%s: %w", "failure in StateAt", err), Index: -1}
	} else {
		var changes []*TransformerResult = nil
		commitMsg := []string{}
		ctxWithTime := time2.WithTimeNow(ctx, time.Now())
		if r.DB != nil && transaction == nil {
			applyErr := TransformerBatchApplyError{
				TransformerError: errors.New("no transaction provided, but DB enabled"),
				Index:            0,
			}
			return nil, nil, nil, &applyErr
		}
		if msg, subChanges, err := RunTransformer(ctxWithTime, transformer, state, transaction); err != nil {
			applyErr := TransformerBatchApplyError{
				TransformerError: err,
				Index:            0,
			}
			return nil, nil, nil, &applyErr
		} else {
			commitMsg = append(commitMsg, msg)
			changes = append(changes, subChanges)
		}
		return commitMsg, state, changes, nil
	}
}

type AppEnv struct {
	App  string
	Env  string
	Team string
}

type RootApp struct {
	Env string
	//argocd/v1alpha1/development2.yaml
}

type TransformerResult struct {
	ChangedApps     []AppEnv
	DeletedRootApps []RootApp
	Commits         *CommitIds
}

type CommitIds struct {
	Previous *git.Oid
	Current  *git.Oid
}

func (r *TransformerResult) AddAppEnv(app string, env string, team string) {
	r.ChangedApps = append(r.ChangedApps, AppEnv{
		App:  app,
		Env:  env,
		Team: team,
	})
}

func (r *TransformerResult) AddRootApp(env string) {
	r.DeletedRootApps = append(r.DeletedRootApps, RootApp{
		Env: env,
	})
}

func (r *TransformerResult) Combine(other *TransformerResult) {
	if other == nil {
		return
	}
	for i := range other.ChangedApps {
		a := other.ChangedApps[i]
		r.AddAppEnv(a.App, a.Env, a.Team)
	}
	for i := range other.DeletedRootApps {
		a := other.DeletedRootApps[i]
		r.AddRootApp(a.Env)
	}
	if r.Commits == nil {
		r.Commits = other.Commits
	}
}

func CombineArray(others []*TransformerResult) *TransformerResult {
	//exhaustruct:ignore
	var r *TransformerResult = &TransformerResult{}
	for i := range others {
		r.Combine(others[i])
	}
	return r
}

func (r *repository) ApplyTransformer(ctx context.Context, transaction *sql.Tx, transformer Transformer) (*TransformerResult, *TransformerBatchApplyError) {
	span, ctx := tracer.StartSpanFromContext(ctx, "ApplyTransformer")
	defer span.Finish()

	commitMsg, state, changes, applyErr := r.ApplyTransformersInternal(ctx, transaction, transformer)
	if applyErr != nil {
		return nil, applyErr
	}
	if err := r.afterTransform(ctx, transaction, *state); err != nil {
		return nil, &TransformerBatchApplyError{TransformerError: fmt.Errorf("%s: %w", "failure in afterTransform", err), Index: -1}
	}

	treeId, insertError := state.Filesystem.(*fs.TreeBuilderFS).Insert()
	if insertError != nil {
		return nil, &TransformerBatchApplyError{TransformerError: insertError, Index: -1}
	}
	committer := &git.Signature{
		Name:  r.config.CommitterName,
		Email: r.config.CommitterEmail,
		When:  time.Now(),
	}

	transformerMetadata := transformer.GetMetadata()
	if transformerMetadata.AuthorEmail == "" || transformerMetadata.AuthorName == "" {
		return nil, &TransformerBatchApplyError{
			TransformerError: fmt.Errorf("transformer metadata is empty"),
			Index:            -1,
		}
	}
	user := auth.User{
		Email:          transformerMetadata.AuthorEmail,
		Name:           transformerMetadata.AuthorName,
		DexAuthContext: nil,
	}

	author := &git.Signature{
		Name:  user.Name,
		Email: user.Email,
		When:  time.Now(),
	}

	var rev *git.Oid
	// the commit can be nil, if it's the first commit in the repo
	if state.Commit != nil {
		rev = state.Commit.Id()
	}
	oldCommitId := rev

	newCommitId, createErr := r.repository.CreateCommitFromIds(
		fmt.Sprintf("refs/heads/%s", r.config.Branch),
		author,
		committer,
		strings.Join(commitMsg, "\n"),
		treeId,
		rev,
	)
	if createErr != nil {
		return nil, &TransformerBatchApplyError{
			TransformerError: fmt.Errorf("%s: %w", "createCommitFromIds failed", createErr),
			Index:            -1,
		}
	}
	result := CombineArray(changes)
	result.Commits = &CommitIds{
		Current:  newCommitId,
		Previous: nil,
	}
	if oldCommitId != nil {
		result.Commits.Previous = oldCommitId
	}
	return result, nil
}

func (r *repository) FetchAndReset(ctx context.Context) error {
	span, ctx := tracer.StartSpanFromContext(ctx, "FetchAndReset")
	defer span.Finish()
	fetchSpec := fmt.Sprintf("+refs/heads/%s:refs/remotes/origin/%s", r.config.Branch, r.config.Branch)
	logger := logger.FromContext(ctx)
	//exhaustruct:ignore
	RemoteCallbacks := git.RemoteCallbacks{
		UpdateTipsCallback: func(refname string, a *git.Oid, b *git.Oid) error {
			logger.Debug("git.fetched",
				zap.String("refname", refname),
				zap.String("revision.new", b.String()),
			)
			return nil
		},
		CredentialsCallback:      r.credentials.CredentialsCallback(ctx),
		CertificateCheckCallback: r.certificates.CertificateCheckCallback(ctx),
	}
	fetchOptions := git.FetchOptions{
		Prune:           git.FetchPruneUnspecified,
		UpdateFetchhead: false,
		DownloadTags:    git.DownloadTagsUnspecified,
		Headers:         nil,
		ProxyOptions: git.ProxyOptions{
			Type: git.ProxyTypeNone,
			Url:  "",
		},
		RemoteCallbacks: RemoteCallbacks,
	}
	err := r.useRemote(func(remote *git.Remote) error {
		return remote.Fetch([]string{fetchSpec}, &fetchOptions, "fetching")
	})
	if err != nil {
		return err
	}
	var zero git.Oid
	var rev *git.Oid = &zero
	if remoteRef, err := r.repository.References.Lookup(fmt.Sprintf("refs/remotes/origin/%s", r.config.Branch)); err != nil {
		var gerr *git.GitError
		if errors.As(err, &gerr) && gerr.Code == git.ErrorCodeNotFound {
			// not found
			// nothing to do
		} else {
			return err
		}
	} else {
		rev = remoteRef.Target()
		if _, err := r.repository.References.Create(fmt.Sprintf("refs/heads/%s", r.config.Branch), rev, true, "reset branch"); err != nil {
			return err
		}
	}
	obj, err := r.repository.Lookup(rev)
	if err != nil {
		return err
	}
	commit, err := obj.AsCommit()
	if err != nil {
		return err
	}
	//exhaustruct:ignore
	err = r.repository.ResetToCommit(commit, git.ResetSoft, &git.CheckoutOptions{Strategy: git.CheckoutForce})
	if err != nil {
		return err
	}
	return nil
}

func (r *repository) Apply(ctx context.Context, tx *sql.Tx, transformers ...Transformer) error {
	for i := range transformers {
		t := transformers[i]
		err := r.ProcessQueueOnce(ctx, t, tx)
		if err != nil {
			return err
		}
	}
	return nil
}

// Push returns an 'error' for typing reasons, really it is always a git.GitError
func (r *repository) Push(ctx context.Context, pushAction func() error) error {

	span, ctx := tracer.StartSpanFromContext(ctx, "Apply")
	defer span.Finish()

	eb := r.backOffProvider()
	return backoff.Retry(
		func() error {
			span, _ := tracer.StartSpanFromContext(ctx, "Push")
			defer span.Finish()
			err := pushAction()
			if err != nil {
				gerr, ok := err.(*git.GitError)
				if ok && gerr.Code == git.ErrorCodeNonFastForward {
					return backoff.Permanent(err)
				}
			}
			return err
		},
		eb,
	)
}

func (r *repository) afterTransform(ctx context.Context, transaction *sql.Tx, state State) error {
	span, ctx := tracer.StartSpanFromContext(ctx, "afterTransform")
	defer span.Finish()

	configs, err := state.GetEnvironmentConfigs()
	if err != nil {
		return err
	}
	for env, config := range configs {
		if config.ArgoCd != nil {
			err := r.updateArgoCdApps(ctx, transaction, &state, env, config)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func (r *repository) updateArgoCdApps(ctx context.Context, transaction *sql.Tx, state *State, env string, config config.EnvironmentConfig) error {
	span, ctx := tracer.StartSpanFromContext(ctx, "updateArgoCdApps")
	defer span.Finish()
	if !r.config.ArgoCdGenerateFiles {
		return nil
	}
	fs := state.Filesystem
	if apps, err := state.GetEnvironmentApplications(ctx, transaction, env); err != nil {
		return err
	} else {
		spanCollectData, ctx := tracer.StartSpanFromContext(ctx, "collectData")
		defer spanCollectData.Finish()
		appData := []argocd.AppData{}
		sort.Strings(apps)
		for _, appName := range apps {
			if err != nil {
				return err
			}
			//team, err := state.DBHandler.DBSelectApp().GetApplicationTeamOwner(ctx, transaction, appName)
			oneAppData, err := state.DBHandler.DBSelectApp(ctx, transaction, appName)
			if err != nil {
				return fmt.Errorf("updateArgoCdApps: could not select app '%s' in db %v", appName, err)
			}
			if oneAppData == nil {
				return fmt.Errorf("skipping app %s because it was not found in the database", appName)
			}
			version, err := state.GetEnvironmentApplicationVersion(ctx, transaction, env, appName)
			if err != nil {
				if errors.Is(err, os.ErrNotExist) {
					// if the app does not exist, we skip it
					// (It may not exist at all, or just hasn't been deployed to this environment yet)
					continue
				}
				return err
			}
			if version == nil || *version == 0 {
				// if nothing is deployed, ignore it
				continue
			}
			appData = append(appData, argocd.AppData{
				AppName:  appName,
				TeamName: oneAppData.Metadata.Team,
			})
		}
		spanCollectData.Finish()

		spanRenderAndWrite, ctx := tracer.StartSpanFromContext(ctx, "RenderAndWrite")
		defer spanRenderAndWrite.Finish()
		if manifests, err := argocd.Render(ctx, r.config.URL, r.config.Branch, config, env, appData); err != nil {
			return err
		} else {
			spanWrite, _ := tracer.StartSpanFromContext(ctx, "Write")
			defer spanWrite.Finish()
			for apiVersion, content := range manifests {
				if err := fs.MkdirAll(fs.Join("argocd", string(apiVersion)), 0777); err != nil {
					return err
				}
				target := fs.Join("argocd", string(apiVersion), fmt.Sprintf("%s.yaml", env))
				if err := util.WriteFile(fs, target, content, 0666); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func (r *repository) State() *State {
	s, err := r.StateAt(nil)
	if err != nil {
		panic(err)
	}
	return s
}

func (r *repository) StateAt(oid *git.Oid) (*State, error) {
	var commit *git.Commit
	if oid == nil {
		if obj, err := r.repository.RevparseSingle(fmt.Sprintf("refs/heads/%s", r.config.Branch)); err != nil {
			var gerr *git.GitError
			if errors.As(err, &gerr) {
				if gerr.Code == git.ErrorCodeNotFound {
					return &State{
						Commit:               nil,
						Filesystem:           fs.NewEmptyTreeBuildFS(r.repository),
						DBHandler:            r.DB,
						ReleaseVersionsLimit: r.config.ReleaseVersionLimit,
					}, nil
				}
			}
			return nil, err
		} else {
			commit, err = obj.AsCommit()
			if err != nil {
				return nil, err
			}
		}
	} else {
		var err error
		commit, err = r.repository.LookupCommit(oid)
		if err != nil {
			return nil, err
		}
	}
	return &State{
		Filesystem:           fs.NewTreeBuildFS(r.repository, commit.TreeId()),
		Commit:               commit,
		ReleaseVersionsLimit: r.config.ReleaseVersionLimit,
		DBHandler:            r.DB,
	}, nil
}

type State struct {
	Filesystem           billy.Filesystem
	Commit               *git.Commit
	ReleaseVersionsLimit uint
	// DbHandler will be nil if the DB is disabled
	DBHandler *db.DBHandler
}

func (s *State) Releases(application string) ([]uint64, error) {
	if entries, err := s.Filesystem.ReadDir(s.Filesystem.Join("applications", application, "releases")); err != nil {
		return nil, err
	} else {
		result := make([]uint64, 0, len(entries))
		for _, e := range entries {
			if i, err := strconv.ParseUint(e.Name(), 10, 64); err != nil {
				// just skip
			} else {
				result = append(result, i)
			}
		}
		return result, nil
	}
}

func (s *State) ReleaseManifests(application string, release uint64) (map[string]string, error) {
	base := s.Filesystem.Join("applications", application, "releases", strconv.FormatUint(release, 10), "environments")
	if entries, err := s.Filesystem.ReadDir(base); err != nil {
		return nil, err
	} else {
		result := make(map[string]string, len(entries))
		for _, e := range entries {
			if buf, err := readFile(s.Filesystem, s.Filesystem.Join(base, e.Name(), "manifests.yaml")); err != nil {
				return nil, err
			} else {
				result[e.Name()] = string(buf)
			}
		}
		return result, nil
	}
}

type Actor struct {
	Name  string
	Email string
}

type Lock struct {
	Message   string
	CreatedBy Actor
	CreatedAt time.Time
}

func readLock(fs billy.Filesystem, lockDir string) (*Lock, error) {
	lock := &Lock{
		Message: "",
		CreatedBy: Actor{
			Name:  "",
			Email: "",
		},
		CreatedAt: time.Time{},
	}

	if cnt, err := readFile(fs, fs.Join(lockDir, "message")); err != nil {
		if !os.IsNotExist(err) {
			return nil, err
		}
	} else {
		lock.Message = string(cnt)
	}

	if cnt, err := readFile(fs, fs.Join(lockDir, "created_by_email")); err != nil {
		if !os.IsNotExist(err) {
			return nil, err
		}
	} else {
		lock.CreatedBy.Email = string(cnt)
	}

	if cnt, err := readFile(fs, fs.Join(lockDir, "created_by_name")); err != nil {
		if !os.IsNotExist(err) {
			return nil, err
		}
	} else {
		lock.CreatedBy.Name = string(cnt)
	}

	if cnt, err := readFile(fs, fs.Join(lockDir, "created_at")); err != nil {
		if !os.IsNotExist(err) {
			return nil, err
		}
	} else {
		if createdAt, err := time.Parse(time.RFC3339, strings.TrimSpace(string(cnt))); err != nil {
			return nil, err
		} else {
			lock.CreatedAt = createdAt
		}
	}

	return lock, nil
}

func (s *State) GetEnvLocksDir(environment string) string {
	return s.Filesystem.Join("environments", environment, "locks")
}

func (s *State) GetEnvLockDir(environment string, lockId string) string {
	return s.Filesystem.Join(s.GetEnvLocksDir(environment), lockId)
}

func (s *State) GetAppLocksDir(environment string, application string) string {
	return s.Filesystem.Join("environments", environment, "applications", application, "locks")
}
func (s *State) GetTeamLocksDir(environment string, team string) string {
	return s.Filesystem.Join("environments", environment, "teams", team, "locks")
}

func (s *State) GetEnvironmentLocksFromDB(ctx context.Context, transaction *sql.Tx, environment string) (map[string]Lock, error) {
	dbLocks, err := s.DBHandler.DBSelectAllEnvironmentLocks(ctx, transaction, environment)
	if err != nil {
		return nil, err
	}
	var lockIds []string
	if dbLocks != nil {
		lockIds = dbLocks.EnvLocks
	}
	locks, err := s.DBHandler.DBSelectEnvironmentLockSet(ctx, transaction, environment, lockIds)

	if err != nil {
		return nil, err
	}
	result := make(map[string]Lock, len(locks))
	for _, lock := range locks {
		genericLock := Lock{
			Message: lock.Metadata.Message,
			CreatedBy: Actor{
				Name:  lock.Metadata.CreatedByName,
				Email: lock.Metadata.CreatedByEmail,
			},
			CreatedAt: lock.Created,
		}
		result[lock.LockID] = genericLock
	}
	return result, nil
}

func (s *State) GetLastRelease(ctx context.Context, fs billy.Filesystem, application string) (uint64, error) {
	var err error
	releasesDir := releasesDirectory(fs, application)
	err = fs.MkdirAll(releasesDir, 0777)
	if err != nil {
		return 0, err
	}
	if entries, err := fs.ReadDir(releasesDir); err != nil {
		return 0, err
	} else {
		var lastRelease uint64 = 0
		for _, e := range entries {
			if i, err := strconv.ParseUint(e.Name(), 10, 64); err != nil {
				logger.FromContext(ctx).Sugar().Warnf("Bad name for release: '%s'\n", e.Name())
			} else {
				if i > lastRelease {
					lastRelease = i
				}
			}
		}
		return lastRelease, nil
	}
}

func (s *State) GetEnvironmentLocks(environment string) (map[string]Lock, error) {
	base := s.GetEnvLocksDir(environment)
	if entries, err := s.Filesystem.ReadDir(base); err != nil {
		return nil, err
	} else {
		result := make(map[string]Lock, len(entries))
		for _, e := range entries {
			if !e.IsDir() {
				return nil, fmt.Errorf("error getting environment locks: found file in the locks directory. run migration script to generate correct metadata")
			}
			if lock, err := readLock(s.Filesystem, s.Filesystem.Join(base, e.Name())); err != nil {
				return nil, err
			} else {
				result[e.Name()] = *lock
			}
		}
		return result, nil
	}
}

func (s *State) GetEnvironmentApplicationLocks(environment, application string) (map[string]Lock, error) {
	base := s.GetAppLocksDir(environment, application)
	if entries, err := s.Filesystem.ReadDir(base); err != nil {
		return nil, err
	} else {
		result := make(map[string]Lock, len(entries))
		for _, e := range entries {
			if !e.IsDir() {
				return nil, fmt.Errorf("error getting application locks: found file in the locks directory. run migration script to generate correct metadata")
			}
			if lock, err := readLock(s.Filesystem, s.Filesystem.Join(base, e.Name())); err != nil {
				return nil, err
			} else {
				result[e.Name()] = *lock
			}
		}
		return result, nil
	}
}
func (s *State) GetEnvironmentTeamLocks(environment, team string) (map[string]Lock, error) {
	base := s.GetTeamLocksDir(environment, team)
	if entries, err := s.Filesystem.ReadDir(base); err != nil {
		return nil, err
	} else {
		result := make(map[string]Lock, len(entries))
		for _, e := range entries {
			if !e.IsDir() {
				return nil, fmt.Errorf("error getting team locks: found file in the locks directory. run migration script to generate correct metadata")
			}
			if lock, err := readLock(s.Filesystem, s.Filesystem.Join(base, e.Name())); err != nil {
				return nil, err
			} else {
				result[e.Name()] = *lock
			}
		}
		return result, nil
	}
}
func (s *State) GetDeploymentMetaData(environment, application string) (string, time.Time, error) {
	base := s.Filesystem.Join("environments", environment, "applications", application)
	author, err := readFile(s.Filesystem, s.Filesystem.Join(base, "deployed_by"))
	if err != nil {
		if os.IsNotExist(err) {
			// for backwards compatibility, we do not return an error here
			return "", time.Time{}, nil
		} else {
			return "", time.Time{}, err
		}
	}

	time_utc, err := readFile(s.Filesystem, s.Filesystem.Join(base, "deployed_at_utc"))
	if err != nil {
		if os.IsNotExist(err) {
			return string(author), time.Time{}, nil
		} else {
			return "", time.Time{}, err
		}
	}

	deployedAt, err := time.Parse("2006-01-02 15:04:05 -0700 MST", strings.TrimSpace(string(time_utc)))
	if err != nil {
		return "", time.Time{}, err
	}

	return string(author), deployedAt, nil
}

func (s *State) DeleteTeamLockIfEmpty(ctx context.Context, environment string, team string) error {
	dir := s.GetTeamLocksDir(environment, team)
	_, err := s.DeleteDirIfEmpty(dir)
	return err
}

func (s *State) DeleteAppLockIfEmpty(ctx context.Context, environment string, application string) error {
	dir := s.GetAppLocksDir(environment, application)
	_, err := s.DeleteDirIfEmpty(dir)
	return err
}

func (s *State) DeleteEnvLockIfEmpty(ctx context.Context, environment string) error {
	dir := s.GetEnvLocksDir(environment)
	_, err := s.DeleteDirIfEmpty(dir)
	return err
}

type SuccessReason int64

const (
	NoReason SuccessReason = iota
	DirDoesNotExist
	DirNotEmpty
)

// DeleteDirIfEmpty if it's empty. If the dir does not exist or is not empty, nothing happens.
// Errors are only returned if the read or delete operations fail.
// Returns SuccessReason for unit testing.
func (s *State) DeleteDirIfEmpty(directoryName string) (SuccessReason, error) {
	fileInfos, err := s.Filesystem.ReadDir(directoryName)
	if err != nil {
		return NoReason, fmt.Errorf("DeleteDirIfEmpty: failed to read directory %q: %w", directoryName, err)
	}
	if fileInfos == nil {
		// directory does not exist, nothing to do
		return DirDoesNotExist, nil
	}
	if len(fileInfos) == 0 {
		// directory exists, and is empty: delete it
		err = s.Filesystem.Remove(directoryName)
		if err != nil {
			return NoReason, fmt.Errorf("DeleteDirIfEmpty: failed to delete directory %q: %w", directoryName, err)
		}
		return NoReason, nil
	}
	return DirNotEmpty, nil
}

func (s *State) GetQueuedVersion(environment string, application string) (*uint64, error) {
	return s.readSymlink(environment, application, queueFileName)
}

func (s *State) DeleteQueuedVersion(environment string, application string) error {
	queuedVersion := s.Filesystem.Join("environments", environment, "applications", application, queueFileName)
	return s.Filesystem.Remove(queuedVersion)
}

func (s *State) DeleteQueuedVersionIfExists(environment string, application string) error {
	queuedVersion, err := s.GetQueuedVersion(environment, application)
	if err != nil {
		return err
	}
	if queuedVersion == nil {
		return nil // nothing to do
	}
	return s.DeleteQueuedVersion(environment, application)
}

func (s *State) GetEnvironmentApplicationVersion(ctx context.Context, transaction *sql.Tx, environment, application string) (*uint64, error) {
	depl, err := s.DBHandler.DBSelectLatestDeployment(ctx, transaction, application, environment)
	if err != nil {
		return nil, err
	}
	if depl == nil || depl.Version == nil {
		return nil, nil
	}
	var v = uint64(*depl.Version)
	return &v, nil
}

// returns nil if there is no file
func (s *State) readSymlink(environment string, application string, symlinkName string) (*uint64, error) {
	version := s.Filesystem.Join("environments", environment, "applications", application, symlinkName)
	if lnk, err := s.Filesystem.Readlink(version); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			// if the link does not exist, we return nil
			return nil, nil
		}
		return nil, fmt.Errorf("failed reading symlink %q: %w", version, err)
	} else {
		target := s.Filesystem.Join("environments", environment, "applications", application, lnk)
		if stat, err := s.Filesystem.Stat(target); err != nil {
			// if the file that the link points to does not exist, that's an error
			return nil, fmt.Errorf("failed stating %q: %w", target, err)
		} else {
			res, err := strconv.ParseUint(stat.Name(), 10, 64)
			return &res, err
		}
	}
}

func (s *State) GetTeamName(application string) (string, error) {
	fs := s.Filesystem

	teamFilePath := fs.Join("applications", application, "team")

	if teamName, err := util.ReadFile(fs, teamFilePath); err != nil {
		return "", err
	} else {
		return string(teamName), nil
	}
}

var InvalidJson = errors.New("JSON file is not valid")

func envExists(envConfigs map[string]config.EnvironmentConfig, envNameToSearchFor string) bool {
	if _, found := envConfigs[envNameToSearchFor]; found {
		return true
	}
	return false
}

func (s *State) GetEnvironmentConfigsAndValidate(ctx context.Context) (map[string]config.EnvironmentConfig, error) {
	logger := logger.FromContext(ctx)
	envConfigs, err := s.GetEnvironmentConfigs()
	if err != nil {
		return nil, err
	}
	if len(envConfigs) == 0 {
		logger.Warn("No environment configurations found. Check git settings like the branch name. Kuberpult cannot operate without environments.")
	}
	for envName, env := range envConfigs {
		if env.Upstream == nil || env.Upstream.Environment == "" {
			continue
		}
		upstreamEnv := env.Upstream.Environment
		if !envExists(envConfigs, upstreamEnv) {
			logger.Warn(fmt.Sprintf("The environment '%s' has upstream '%s' configured, but the environment '%s' does not exist.", envName, upstreamEnv, upstreamEnv))
		}
	}
	envGroups := mapper.MapEnvironmentsToGroups(envConfigs)
	for _, group := range envGroups {
		grpDist := group.Environments[0].DistanceToUpstream
		for _, env := range group.Environments {
			if env.DistanceToUpstream != grpDist {
				logger.Warn(fmt.Sprintf("The environment group '%s' has multiple environments setup with different distances to upstream", group.EnvironmentGroupName))
			}
		}
	}
	return envConfigs, err
}

func (s *State) GetEnvironmentConfigs() (map[string]config.EnvironmentConfig, error) {
	envs, err := s.Filesystem.ReadDir("environments")
	if err != nil {
		return nil, err
	}
	result := map[string]config.EnvironmentConfig{}
	for _, env := range envs {
		c, err := s.GetEnvironmentConfig(env.Name())
		if err != nil {
			return nil, err

		}
		result[env.Name()] = *c
	}
	return result, nil
}

func (s *State) GetEnvironmentConfig(environmentName string) (*config.EnvironmentConfig, error) {
	fileName := s.Filesystem.Join("environments", environmentName, "config.json")
	var config config.EnvironmentConfig
	if err := decodeJsonFile(s.Filesystem, fileName, &config); err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return nil, fmt.Errorf("%s : %w", fileName, InvalidJson)
		}
	}
	return &config, nil
}

func (s *State) GetEnvironmentConfigsForGroup(envGroup string) ([]string, error) {
	allEnvConfigs, err := s.GetEnvironmentConfigs()
	if err != nil {
		return nil, err
	}
	groupEnvNames := []string{}
	for env := range allEnvConfigs {
		envConfig := allEnvConfigs[env]
		g := envConfig.EnvironmentGroup
		if g != nil && *g == envGroup {
			groupEnvNames = append(groupEnvNames, env)
		}
	}
	if len(groupEnvNames) == 0 {
		return nil, fmt.Errorf("No environment found with given group '%s'", envGroup)
	}
	sort.Strings(groupEnvNames)
	return groupEnvNames, nil
}

func (s *State) GetEnvironmentApplications(ctx context.Context, transaction *sql.Tx, environment string) ([]string, error) {
	applications, err := s.DBHandler.DBSelectAllApplications(ctx, transaction)
	if err != nil {
		return nil, err
	}
	if applications == nil {
		return make([]string, 0), nil
	}
	return applications.Apps, nil
}

// GetApplicationsFromFile returns apps from the filesystem
func (s *State) GetApplicationsFromFile() ([]string, error) {
	return names(s.Filesystem, "applications")
}

func (s *State) GetApplicationReleasesFromFile(application string) ([]uint64, error) {
	if ns, err := names(s.Filesystem, s.Filesystem.Join("applications", application, "releases")); err != nil {
		return nil, err
	} else {
		result := make([]uint64, 0, len(ns))
		for _, n := range ns {
			if i, err := strconv.ParseUint(n, 10, 64); err == nil {
				result = append(result, i)
			}
		}
		sort.Slice(result, func(i, j int) bool {
			return result[i] < result[j]
		})
		return result, nil
	}
}

type Release struct {
	Version uint64
	/**
	"UndeployVersion=true" means that this version is empty, and has no manifest that could be deployed.
	It is intended to help cleanup old services within the normal release cycle (e.g. dev->staging->production).
	*/
	UndeployVersion bool
	SourceAuthor    string
	SourceCommitId  string
	SourceMessage   string
	CreatedAt       time.Time
	DisplayVersion  string
}

func (rel *Release) ToProto() *api.Release {
	if rel == nil {
		return nil
	}
	return &api.Release{
		PrNumber:        extractPrNumber(rel.SourceMessage),
		Version:         rel.Version,
		SourceAuthor:    rel.SourceAuthor,
		SourceCommitId:  rel.SourceCommitId,
		SourceMessage:   rel.SourceMessage,
		UndeployVersion: rel.UndeployVersion,
		CreatedAt:       timestamppb.New(rel.CreatedAt),
		DisplayVersion:  rel.DisplayVersion,
		IsMinor:         false,
		IsPrepublish:    false,
	}
}

func extractPrNumber(sourceMessage string) string {
	re := regexp.MustCompile(`\(#(\d+)\)`)
	res := re.FindAllStringSubmatch(sourceMessage, -1)

	if len(res) == 0 {
		return ""
	} else {
		return res[len(res)-1][1]
	}
}

func (s *State) IsUndeployVersion(application string, version uint64) (bool, error) {
	base := releasesDirectoryWithVersion(s.Filesystem, application, version)
	_, err := s.Filesystem.Stat(base)
	if err != nil {
		return false, wrapFileError(err, base, "could not call stat")
	}
	if _, err := readFile(s.Filesystem, s.Filesystem.Join(base, "undeploy")); err != nil {
		if !os.IsNotExist(err) {
			return false, err
		}
		return false, nil
	}
	return true, nil
}

func (s *State) GetApplicationRelease(application string, version uint64) (*Release, error) {
	base := releasesDirectoryWithVersion(s.Filesystem, application, version)
	_, err := s.Filesystem.Stat(base)
	if err != nil {
		return nil, wrapFileError(err, base, "could not call stat")
	}
	release := Release{
		Version:         version,
		UndeployVersion: false,
		SourceAuthor:    "",
		SourceCommitId:  "",
		SourceMessage:   "",
		CreatedAt:       time.Time{},
		DisplayVersion:  "",
	}
	if cnt, err := readFile(s.Filesystem, s.Filesystem.Join(base, "source_commit_id")); err != nil {
		if !os.IsNotExist(err) {
			return nil, err
		}
	} else {
		release.SourceCommitId = string(cnt)
	}
	if cnt, err := readFile(s.Filesystem, s.Filesystem.Join(base, "source_author")); err != nil {
		if !os.IsNotExist(err) {
			return nil, err
		}
	} else {
		release.SourceAuthor = string(cnt)
	}
	if cnt, err := readFile(s.Filesystem, s.Filesystem.Join(base, "source_message")); err != nil {
		if !os.IsNotExist(err) {
			return nil, err
		}
	} else {
		release.SourceMessage = string(cnt)
	}
	if displayVersion, err := readFile(s.Filesystem, s.Filesystem.Join(base, "display_version")); err != nil {
		if !os.IsNotExist(err) {
			return nil, err
		}
		release.DisplayVersion = ""
	} else {
		release.DisplayVersion = string(displayVersion)
	}
	isUndeploy, err := s.IsUndeployVersion(application, version)
	if err != nil {
		return nil, err
	}
	release.UndeployVersion = isUndeploy
	if cnt, err := readFile(s.Filesystem, s.Filesystem.Join(base, "created_at")); err != nil {
		if !os.IsNotExist(err) {
			return nil, err
		}
	} else {
		if releaseTime, err := time.Parse(time.RFC3339, strings.TrimSpace(string(cnt))); err != nil {
			return nil, err
		} else {
			release.CreatedAt = releaseTime
		}
	}
	return &release, nil
}

func (s *State) GetApplicationReleaseManifests(application string, version uint64) (map[string]*api.Manifest, error) {
	dir := manifestDirectoryWithReleasesVersion(s.Filesystem, application, version)

	entries, err := s.Filesystem.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("reading manifest directory: %w", err)
	}
	manifests := map[string]*api.Manifest{}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		manifestPath := filepath.Join(dir, entry.Name(), "manifests.yaml")
		file, err := s.Filesystem.Open(manifestPath)
		if err != nil {
			return nil, fmt.Errorf("failed to open %s: %w", manifestPath, err)
		}
		content, err := io.ReadAll(file)
		if err != nil {
			return nil, fmt.Errorf("failed to read %s: %w", manifestPath, err)
		}

		manifests[entry.Name()] = &api.Manifest{
			Environment: entry.Name(),
			Content:     string(content),
		}
	}
	return manifests, nil
}

func (s *State) GetApplicationTeamOwner(ctx context.Context, transaction *sql.Tx, application string) (string, error) {
	app, err := s.DBHandler.DBSelectApp(ctx, transaction, application)
	if err != nil {
		return "", fmt.Errorf("could not get team of app '%s': %v", application, err)
	}
	if app == nil {
		return "", fmt.Errorf("could not get team of app '%s' - could not find app", application)
	}
	return app.Metadata.Team, nil
}

func (s *State) GetApplicationSourceRepoUrl(application string) (string, error) {
	appDir := applicationDirectory(s.Filesystem, application)
	appSourceRepoUrl := s.Filesystem.Join(appDir, "sourceRepoUrl")

	if url, err := readFile(s.Filesystem, appSourceRepoUrl); err != nil {
		if os.IsNotExist(err) {
			return "", nil
		} else {
			return "", fmt.Errorf("error while reading sourceRepoUrl file for application %v found: %w", application, err)
		}
	} else {
		return string(url), nil
	}
}

func names(fs billy.Filesystem, path string) ([]string, error) {
	files, err := fs.ReadDir(path)
	if err != nil {
		return nil, err
	}
	result := make([]string, 0, len(files))
	for _, app := range files {
		result = append(result, app.Name())
	}
	return result, nil
}

func decodeJsonFile(fs billy.Filesystem, path string, out interface{}) error {
	if file, err := fs.Open(path); err != nil {
		return wrapFileError(err, path, "could not decode json file")
	} else {
		defer file.Close()
		dec := json.NewDecoder(file)
		return dec.Decode(out)
	}
}

func readFile(fs billy.Filesystem, path string) ([]byte, error) {
	if file, err := fs.Open(path); err != nil {
		return nil, err
	} else {
		defer file.Close()
		return io.ReadAll(file)
	}
}

// ProcessQueue checks if there is something in the queue
// deploys if necessary
// deletes the queue
func (s *State) ProcessQueue(ctx context.Context, transaction *sql.Tx, fs billy.Filesystem, environment string, application string) (string, error) {
	queuedVersion, err := s.GetQueuedVersion(environment, application)
	queueDeploymentMessage := ""
	if err != nil {
		// could not read queued version.
		return "", err
	} else {
		if queuedVersion == nil {
			// if there is no version queued, that's not an issue, just do nothing:
			return "", nil
		}

		currentlyDeployedVersion, err := s.GetEnvironmentApplicationVersion(ctx, transaction, environment, application)
		if err != nil {
			return "", err
		}

		if currentlyDeployedVersion != nil && *queuedVersion == *currentlyDeployedVersion {
			// delete queue, it's outdated! But if we can't, that's not really a problem, as it would be overwritten
			// whenever the next deployment happens:
			err = s.DeleteQueuedVersion(environment, application)
			return fmt.Sprintf("deleted queued version %d because it was already deployed. app=%q env=%q", *queuedVersion, application, environment), err
		}
	}
	return queueDeploymentMessage, nil
}

func GetTags(cfg RepositoryConfig, repoName string, ctx context.Context) (tags []*api.TagData, err error) {
	repo, err := openOrCreate(repoName)
	if err != nil {
		return nil, fmt.Errorf("unable to open/create repo: %v", err)
	}

	var credentials *credentialsStore
	var certificates *certificateStore
	if strings.HasPrefix(cfg.URL, "./") || strings.HasPrefix(cfg.URL, "/") {
	} else {
		credentials, err = cfg.Credentials.load()
		if err != nil {
			return nil, fmt.Errorf("failure to load credentials: %v", err)
		}
		certificates, err = cfg.Certificates.load()
		if err != nil {
			return nil, fmt.Errorf("failure to load certificates: %v", err)
		}
	}

	fetchSpec := fmt.Sprintf("+refs/heads/%s:refs/remotes/origin/%s", cfg.Branch, cfg.Branch)
	//exhaustruct:ignore
	RemoteCallbacks := git.RemoteCallbacks{
		CredentialsCallback:      credentials.CredentialsCallback(ctx),
		CertificateCheckCallback: certificates.CertificateCheckCallback(ctx),
	}
	fetchOptions := git.FetchOptions{
		Prune:           git.FetchPruneUnspecified,
		UpdateFetchhead: false,
		Headers:         nil,
		ProxyOptions: git.ProxyOptions{
			Type: git.ProxyTypeNone,
			Url:  "",
		},
		RemoteCallbacks: RemoteCallbacks,
		DownloadTags:    git.DownloadTagsAll,
	}
	remote, err := repo.Remotes.CreateAnonymous(cfg.URL)
	if err != nil {
		return nil, fmt.Errorf("failure to create anonymous remote: %v", err)
	}
	err = remote.Fetch([]string{fetchSpec}, &fetchOptions, "fetching")
	if err != nil {
		return nil, fmt.Errorf("failure to fetch: %v", err)
	}

	tagsList, err := repo.Tags.List()
	if err != nil {
		return nil, fmt.Errorf("unable to list tags: %v", err)
	}

	sort.Strings(tagsList)
	iters, err := repo.NewReferenceIteratorGlob("refs/tags/*")
	if err != nil {
		return nil, fmt.Errorf("unable to get list of tags: %v", err)
	}
	for {
		tagObject, err := iters.Next()
		if err != nil {
			break
		}
		tagRef, lookupErr := repo.LookupTag(tagObject.Target())
		if lookupErr != nil {
			tagCommit, err := repo.LookupCommit(tagObject.Target())
			// If LookupTag fails, fallback to LookupCommit
			// to cover all tags, annotated and lightweight
			if err != nil {
				return nil, fmt.Errorf("unable to lookup tag [%s]: %v - original err: %v", tagObject.Name(), err, lookupErr)
			}
			tags = append(tags, &api.TagData{Tag: tagObject.Name(), CommitId: tagCommit.Id().String()})

		} else {
			tagCommit, err := repo.LookupCommit(tagRef.TargetId())
			if err != nil {
				return nil, fmt.Errorf("unable to lookup tag [%s]: %v", tagObject.Name(), err)
			}
			tags = append(tags, &api.TagData{Tag: tagObject.Name(), CommitId: tagCommit.Id().String()})
		}
	}

	return tags, nil
}
