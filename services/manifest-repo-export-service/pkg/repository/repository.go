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
	"sync"
	"time"

	"github.com/DataDog/datadog-go/v5/statsd"
	backoff "github.com/cenkalti/backoff/v4"
	billy "github.com/go-git/go-billy/v5"
	"github.com/go-git/go-billy/v5/util"
	git "github.com/libgit2/git2go/v34"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"
	"google.golang.org/protobuf/types/known/timestamppb"
	"gopkg.in/DataDog/dd-trace-go.v1/ddtrace/tracer"

	api "github.com/freiheit-com/kuberpult/pkg/api/v1"
	"github.com/freiheit-com/kuberpult/pkg/auth"
	"github.com/freiheit-com/kuberpult/pkg/config"
	"github.com/freiheit-com/kuberpult/pkg/db"
	"github.com/freiheit-com/kuberpult/pkg/event"
	"github.com/freiheit-com/kuberpult/pkg/logger"
	"github.com/freiheit-com/kuberpult/pkg/mapper"
	time2 "github.com/freiheit-com/kuberpult/pkg/time"
	"github.com/freiheit-com/kuberpult/pkg/tracing"
	"github.com/freiheit-com/kuberpult/pkg/types"
	"github.com/freiheit-com/kuberpult/pkg/valid"
	"github.com/freiheit-com/kuberpult/services/manifest-repo-export-service/pkg/argocd"
	"github.com/freiheit-com/kuberpult/services/manifest-repo-export-service/pkg/db_history"
	"github.com/freiheit-com/kuberpult/services/manifest-repo-export-service/pkg/fs"
	"github.com/freiheit-com/kuberpult/services/manifest-repo-export-service/pkg/notify"
	"github.com/freiheit-com/kuberpult/services/manifest-repo-export-service/pkg/sqlitestore"
)

// A Repository provides a multiple reader / single writer access to a git repository.
type Repository interface {
	Apply(ctx context.Context, tx *sql.Tx, transformers ...Transformer) error
	Push(ctx context.Context, pushAction func() error) error
	PushTag(ctx context.Context, tag types.GitTag) error
	ApplyTransformersInternal(ctx context.Context, transaction *sql.Tx, transformer Transformer) ([]string, *State, *TransformerResult, *TransformerBatchApplyError)
	State() *State
	StateAt(oid *git.Oid) (*State, error)
	FetchAndReset(ctx context.Context) error
	PushRepo(ctx context.Context) error
	GetHeadCommitId() (*git.Oid, error)
	FixCommitsTimestamp(ctx context.Context, state State) error
	Notify() *notify.Notify
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

	notify notify.Notify

	backOffProvider func() backoff.BackOff

	DB *db.DBHandler

	ddMetrics statsd.ClientInterface

	ArgoProjectNames *argocd.AllArgoProjectNameOverrides
}

var _ Repository = &repository{} // ensure interface is implemented

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

	DBHandler *db.DBHandler

	ReleaseVersionLimit uint

	MinimizeExportedData bool

	DDMetrics statsd.ClientInterface
	TagsPath  string

	ArgoCdGenerateFiles bool
	ArgoProjectNames    *argocd.AllArgoProjectNameOverrides
}

func openOrCreate(path string) (*git.Repository, error) {
	repo2, err := git.OpenRepositoryExtended(path, git.RepositoryOpenNoSearch, path)
	if err != nil {
		var gerr *git.GitError
		if errors.As(err, &gerr) {
			if gerr.Code == git.ErrorCodeNotFound {
				err = os.MkdirAll(path, 0777)
				if err != nil {
					return nil, fmt.Errorf("could not mkdirAll %s: %v", path, err)
				}
				repo2, err = git.InitRepository(path, true)
				if err != nil {
					return nil, fmt.Errorf("init repository %s: %v", path, err)
				}
			} else {
				return nil, fmt.Errorf("other error %s: %v", path, err)
			}
		} else {
			return nil, fmt.Errorf("non-git error %s: %v", path, err)
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
	return repo2, nil
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
				config:           &cfg,
				credentials:      credentials,
				certificates:     certificates,
				repository:       repo2,
				backOffProvider:  defaultBackOffProvider,
				DB:               cfg.DBHandler,
				notify:           notify.Notify{},
				ddMetrics:        cfg.DDMetrics,
				ArgoProjectNames: cfg.ArgoProjectNames,
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

			if state == nil || state.DBHandler == nil {
				return nil, fmt.Errorf("no database configured")
			}
			// Check configuration for errors and abort early if any:
			err = state.DBHandler.WithTransaction(ctx, true, func(ctx context.Context, transaction *sql.Tx) error {
				_, err = state.GetEnvironmentConfigsAndValidate(ctx, transaction)
				return err
			})
			if err != nil {
				return nil, err
			}

			return result, nil
		}
	}
}

func (r *repository) applyTransformerBatches(ctx context.Context, transformer Transformer, allowFetchAndReset bool, transaction *sql.Tx) (*TransformerResult, error) {
	span, ctx := tracer.StartSpanFromContext(ctx, "applyTransformerBatches")
	defer span.Finish()
	span.SetTag("allowFetchAndReset", allowFetchAndReset)
	//exhaustruct:ignore
	var changes = &TransformerResult{}
	subChanges, applyErr := r.ApplyTransformer(ctx, transaction, transformer)
	changes.Combine(subChanges)
	if applyErr != nil {
		if errors.Is(applyErr.TransformerError, ErrInvalidJson) && allowFetchAndReset {
			// Invalid state. fetch and reset and redo
			err := r.FetchAndReset(ctx)
			if err != nil {
				return nil, err
			}
			return r.applyTransformerBatches(ctx, transformer, false, transaction)
		} else {
			return nil, applyErr
		}
	}
	return changes, nil
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
		defer logger.LogPanics(true)
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
func commitPushUpdate(branch string, success *bool) git.PushUpdateReferenceCallback {
	return func(refName string, status string) error {
		var expectedRefName = fmt.Sprintf("refs/heads/%s", branch)
		// if we were successful the status is empty and the ref contains our branch:
		*success = refName == expectedRefName && status == ""
		return nil
	}
}

// TagPushResult is there just for logging purposes
type TagPushResult struct {
	Success         bool   // true if the RefName is what we expect
	ActualRefName   string // the actual RefName as supplied by the callback
	ExpectedRefName string // the actual RefName as supplied by the callback
	Status          string // the actual status as supplied by the callback
}

// tagPushUpdate is the same as commitPushUpdate, but expects a tag reference
func tagPushUpdate(_ context.Context, gitTag types.GitTag, result *TagPushResult) git.PushUpdateReferenceCallback {
	return func(actualRefName string, status string) error {
		var expectedRefName = fmt.Sprintf("refs/tags/%s", gitTag)
		// here we store interesting data for logging:
		result.ActualRefName = actualRefName
		result.ExpectedRefName = expectedRefName
		result.Status = status
		// if we were successful the status is empty and the ref contains our gitTag:
		result.Success = actualRefName == expectedRefName && status == ""
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

func PushTagsActionCallback(_ context.Context, pushOptions git.PushOptions, r *repository, tagName types.GitTag) PushActionFunc {
	return func() error {
		return r.useRemote(func(remote *git.Remote) error {
			return remote.Push([]string{fmt.Sprintf("refs/tags/%s:refs/tags/%s", tagName, tagName)}, &pushOptions)
		})
	}
}

type PushUpdateFunc func(string, *bool) git.PushUpdateReferenceCallback

func (r *repository) ProcessQueueOnce(ctx context.Context, t Transformer, tx *sql.Tx) error {
	span, ctx := tracer.StartSpanFromContext(ctx, "ProcessQueueOnce")
	defer span.Finish()

	log := logger.FromContext(ctx).Sugar()

	// Apply the items
	apply := func() (*TransformerResult, error) {
		changes, err := r.applyTransformerBatches(ctx, t, true, tx)
		if err != nil {
			log.Warnf("rolling back transaction because of %v", err)
			return nil, err
		}
		return changes, nil
	}

	_, err := apply()
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
		PushUpdateReferenceCallback: commitPushUpdate(r.config.Branch, &pushSuccess),
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

func (r *repository) GetHeadCommitId() (*git.Oid, error) {
	branchHead := fmt.Sprintf("refs/heads/%s", r.config.Branch)
	ref, err := r.repository.References.Lookup(branchHead)
	if err != nil {
		return nil, fmt.Errorf("error fetching reference \"%s\": %v", branchHead, err)
	}
	return ref.Target(), nil
}

func (r *repository) ApplyTransformersInternal(ctx context.Context, transaction *sql.Tx, transformer Transformer) ([]string, *State, *TransformerResult, *TransformerBatchApplyError) {
	span, ctx := tracer.StartSpanFromContext(ctx, "ApplyTransformersInternal")
	defer span.Finish()
	if state, err := r.StateAt(nil); err != nil {
		return nil, nil, nil, &TransformerBatchApplyError{TransformerError: fmt.Errorf("%s: %w", "failure in StateAt", err), Index: -1}
	} else {
		subChanges := &TransformerResult{}
		commitMsg := []string{}
		ctxWithTime := time2.WithTimeNow(ctx, time.Now())
		if r.DB != nil && transaction == nil {
			applyErr := TransformerBatchApplyError{
				TransformerError: errors.New("no transaction provided, but DB enabled"),
				Index:            0,
			}
			return nil, nil, nil, &applyErr
		}
		msg, subChanges, err := RunTransformer(ctxWithTime, transformer, state, transaction, r.config.MinimizeExportedData)
		if err != nil {
			applyErr := TransformerBatchApplyError{
				TransformerError: err,
				Index:            0,
			}
			return nil, nil, nil, &applyErr
		} else {
			commitMsg = append(commitMsg, msg)
		}
		return commitMsg, state, subChanges, nil
	}
}

func (s *State) WriteAllCommitEvents(ctx context.Context, transaction *sql.Tx, dbHandler *db.DBHandler) error {
	ddSpan, _ := tracer.StartSpanFromContext(ctx, "WriteAllCommitEvents")
	defer ddSpan.Finish()
	fs := s.Filesystem
	allCommitsPath := "commits"
	commitPrefixes, err := fs.ReadDir(allCommitsPath)
	if err != nil {
		return fmt.Errorf("could not read commits dir: %s - error: %w", allCommitsPath, err)
	}
	for _, currentPrefix := range commitPrefixes {
		currentpath := fs.Join(allCommitsPath, currentPrefix.Name())
		commitSuffixes, err := fs.ReadDir(currentpath)
		if err != nil {
			return fmt.Errorf("could not read commit directory '%s': %w", currentpath, err)
		}
		for _, currentSuffix := range commitSuffixes {
			commitID := strings.Join([]string{currentPrefix.Name(), currentSuffix.Name()}, "")
			currentpath := fs.Join(fs.Join(currentpath, currentSuffix.Name(), "events"))
			potentialEventDirs, err := fs.ReadDir(currentpath)
			if err != nil {
				return fmt.Errorf("could not read events directory '%s': %w", currentpath, err)
			}
			for i := range potentialEventDirs {
				oneEventDir := potentialEventDirs[i]
				if oneEventDir.IsDir() {
					fileName := oneEventDir.Name()

					eType, err := readFile(fs, fs.Join(fs.Join(currentpath, fileName), "eventType"))

					if err != nil {
						return fmt.Errorf("could not read event type '%s': %w", fs.Join(currentpath, fileName), err)
					}

					fsEvent, err := event.Read(fs, fs.Join(currentpath, fileName))
					if err != nil {
						return fmt.Errorf("could not read events %w", err)
					}
					currentEvent := event.DBEventGo{
						EventData: fsEvent,
						EventMetadata: event.Metadata{
							Uuid:           fileName,
							EventType:      string(eType),
							ReleaseVersion: 0, // don't care about release version for this event
						},
					}
					eventJson, err := json.Marshal(currentEvent)
					if err != nil {
						return fmt.Errorf("could not marshal event: %w", err)
					}
					err = dbHandler.WriteEvent(ctx, transaction, 0, currentEvent.EventMetadata.Uuid, event.EventType(currentEvent.EventMetadata.EventType), commitID, eventJson)
					if err != nil {
						return fmt.Errorf("error writing existing event version: %w", err)
					}
				}
			}
		}
	}
	return nil
}

// AppEnvToRender is an app/env combination that has been changed in such a way that it requires us to render them again.
// For example a DeployApplicationVersion would add its app and env here,
// while none of the Lock transformers would add anything.
type AppEnvToRender struct {
	App string
	Env types.EnvName
}

// EnvironmentToRender stands for an environment that needs to be rendered again after the transformer is done.
// This means it was added or changed, but not deleted,
// because deleted environments should not be rendered.
type EnvironmentToRender struct {
	Env types.EnvName
	//argocd/v1alpha1/development2.yaml
}

type TransformerResult struct {
	AppEnvsToRender []AppEnvToRender

	EnvironmentsToRender []EnvironmentToRender
	Commits              *CommitIds
}

type CommitIds struct {
	Previous *git.Oid
	Current  *git.Oid
}

func (r *TransformerResult) AddAppEnv(app string, env types.EnvName) {
	r.AppEnvsToRender = append(r.AppEnvsToRender, AppEnvToRender{
		App: app,
		Env: env,
	})
}

func (r *TransformerResult) AddEnvironmentDeletion(env types.EnvName) {
	r.EnvironmentsToRender = append(r.EnvironmentsToRender, EnvironmentToRender{
		Env: env,
	})
}

func (r *TransformerResult) Combine(other *TransformerResult) {
	if other == nil {
		return
	}
	for i := range other.AppEnvsToRender {
		a := other.AppEnvsToRender[i]
		r.AddAppEnv(a.App, a.Env)
	}
	for i := range other.EnvironmentsToRender {
		a := other.EnvironmentsToRender[i]
		r.AddEnvironmentDeletion(a.Env)
	}
	if r.Commits == nil {
		r.Commits = other.Commits
	}
}

// CalculateChangedEnvironments returns a map with all environments that have been changed in the current transformer
func (r *TransformerResult) CalculateChangedEnvironments() map[types.EnvName]struct{} {
	result := map[types.EnvName]struct{}{}
	for _, changed := range r.AppEnvsToRender {
		result[changed.Env] = struct{}{}
	}
	for _, changed := range r.EnvironmentsToRender {
		result[changed.Env] = struct{}{}
	}
	return result
}

func (r *repository) ApplyTransformer(ctx context.Context, transaction *sql.Tx, transformer Transformer) (*TransformerResult, *TransformerBatchApplyError) {
	span, ctx := tracer.StartSpanFromContext(ctx, "ApplyTransformer")
	defer span.Finish()

	commitMsg, state, result, applyErr := r.ApplyTransformersInternal(ctx, transaction, transformer)
	if applyErr != nil {
		return nil, applyErr
	}

	if r.shouldCreateNewCommit(commitMsg) {
		if err := r.afterTransform(ctx, transaction, *state, transformer.GetCreationTimestamp(), result.CalculateChangedEnvironments()); err != nil {
			return nil, &TransformerBatchApplyError{TransformerError: fmt.Errorf("%s: %w", "failure in afterTransform", err), Index: -1}
		}

		oldCommitId, newCommitId, applyError := r.createCommit(ctx, state, transformer, commitMsg)
		if applyError != nil {
			return nil, applyError
		}

		result.Commits = &CommitIds{
			Current:  newCommitId,
			Previous: nil,
		}
		if oldCommitId != nil {
			result.Commits.Previous = oldCommitId
		}
	}

	return result, nil
}

func (r *repository) createCommit(ctx context.Context, state *State, transformer Transformer, commitMsg []string) (_ *git.Oid, _ *git.Oid, resultError *TransformerBatchApplyError) {
	span, ctx := tracer.StartSpanFromContext(ctx, "createCommit")
	defer func() {
		// Casting our TransformerBatchApplyError to error yields non-nil, even if it was nil.
		// We have to avoid this by manually checking for nil:
		if resultError == nil {
			span.Finish()
		} else {
			span.Finish(tracer.WithError(resultError))
		}
	}()

	insertSpan, _ := tracer.StartSpanFromContext(ctx, "fsInsert")
	treeId, insertError := state.Filesystem.(*fs.TreeBuilderFS).Insert()
	insertSpan.Finish(tracer.WithError(insertError))
	if insertError != nil {
		return nil, nil, &TransformerBatchApplyError{TransformerError: insertError, Index: -1}
	}

	committer := r.makeGitSignature()

	transformerMetadata := transformer.GetMetadata()
	if transformerMetadata.AuthorEmail == "" || transformerMetadata.AuthorName == "" {
		return nil, nil, &TransformerBatchApplyError{
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
		return nil, nil, &TransformerBatchApplyError{
			TransformerError: fmt.Errorf("%s: %w", "createCommitFromIds failed", createErr),
			Index:            -1,
		}
	}
	return oldCommitId, newCommitId, nil
}

func (r *repository) makeGitSignature() *git.Signature {
	return &git.Signature{
		Name:  r.config.CommitterName,
		Email: r.config.CommitterEmail,
		When:  time.Now(),
	}
}

func (r *repository) shouldCreateNewCommit(commitMessages []string) bool {
	if !r.config.MinimizeExportedData {
		return true
	}
	for _, currCommitMessage := range commitMessages {
		if !strings.Contains(currCommitMessage, NoOpMessage) { //Transformers that generate no commits always return a message beginning with $NoOpMessage
			return true
		}
	}
	return false
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
	var rev = &zero
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

func (r *repository) PushTag(ctx context.Context, tag types.GitTag) error {
	span, ctx := tracer.StartSpanFromContext(ctx, "PushTag")
	defer span.Finish()

	currentCommit, err := r.GetHeadCommitId()
	if err != nil {
		return fmt.Errorf("getHeadCommit: %w", err)
	}
	lookedUpCommit, err := r.repository.LookupCommit(currentCommit)
	if err != nil {
		return fmt.Errorf("lookupCommit: %w", err)
	}

	sig := r.makeGitSignature()
	tagMessage := fmt.Sprintf("Kuberpult-generated tag %s", tag)
	_, err = r.repository.Tags.Create(string(tag), lookedUpCommit, sig, tagMessage)
	if err != nil {
		return fmt.Errorf("tag.Create: %w", err)
	}
	var pushResult = TagPushResult{}
	//exhaustruct:ignore
	RemoteCallbacks := git.RemoteCallbacks{
		CredentialsCallback:         r.credentials.CredentialsCallback(ctx),
		CertificateCheckCallback:    r.certificates.CertificateCheckCallback(ctx),
		PushUpdateReferenceCallback: tagPushUpdate(ctx, tag, &pushResult),
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
	err = r.Push(ctx, PushTagsActionCallback(ctx, pushOptions, r, tag))
	if err != nil {
		return fmt.Errorf("push: %w", err)
	}
	if !pushResult.Success {
		return fmt.Errorf("push ran with success='%v' Status='%s', actualRefName='%s', expectedRefName='%s'", pushResult.Success, pushResult.Status, pushResult.ActualRefName, pushResult.ExpectedRefName)
	}
	return nil
}

func (r *repository) afterTransform(ctx context.Context, transaction *sql.Tx, state State, ts time.Time, changedEnvironments map[types.EnvName]struct{}) (err error) {
	span, ctx := tracer.StartSpanFromContext(ctx, "afterTransform")
	defer func() {
		span.Finish(tracer.WithError(err))
	}()

	configs, err := state.GetAllEnvironmentConfigsFromDBAtTimestamp(ctx, transaction, ts)
	if err != nil {
		return err
	}

	errorGroup, ctx := errgroup.WithContext(ctx)
	fsMutex := sync.Mutex{}
	skippedEnvs := []types.EnvName{}
	renderedEnvs := []types.EnvName{}
	for env, config := range configs {
		if config.ArgoCd != nil || config.ArgoCdConfigs != nil {
			_, envHasChanged := changedEnvironments[env]
			if envHasChanged {
				errorGroup.Go(func() error {
					return r.State().DBHandler.WithTransaction(ctx, true, func(ctx context.Context, tx *sql.Tx) error {
						return r.updateArgoCdApps(ctx, tx, &state, env, config, ts, &fsMutex)
					})
				})
				renderedEnvs = append(renderedEnvs, env)
			} else {
				skippedEnvs = append(skippedEnvs, env)
			}
		}
	}
	logger.FromContext(ctx).Info("rendering of environments",
		zap.Strings("skippedEnvs", types.EnvNamesToStrings(skippedEnvs)),
		zap.Strings("renderedEnvs", types.EnvNamesToStrings(renderedEnvs)),
	)
	return errorGroup.Wait()
}

func isAAEnv(config config.EnvironmentConfig) bool {
	return config.ArgoCdConfigs != nil
}

func (r *repository) updateArgoCdApps(ctx context.Context, transaction *sql.Tx, state *State, env types.EnvName, cfg config.EnvironmentConfig, ts time.Time, fsMutex *sync.Mutex) (err error) {
	span, ctx := tracer.StartSpanFromContext(ctx, "updateArgoCdApps")
	defer func() {
		span.Finish(tracer.WithError(err))
	}()
	span.SetTag("environment", string(env))
	if !r.config.ArgoCdGenerateFiles {
		return nil
	}
	if isAAEnv(cfg) {
		for _, currentArgoCdConfiguration := range cfg.ArgoCdConfigs.ArgoCdConfigurations {
			err := r.processApp(ctx, transaction, state, env, cfg.ArgoCdConfigs.CommonEnvPrefix, currentArgoCdConfiguration, true, ts, fsMutex)
			if err != nil {
				return err
			}
		}
	} else {
		if cfg.ArgoCd == nil && (cfg.ArgoCdConfigs == nil || len(cfg.ArgoCdConfigs.ArgoCdConfigurations) == 0) {
			logger.FromContext(ctx).Sugar().Warnf("No argo cd configuration found for environment %q.", env)
			return nil
		}
		var conf *config.EnvironmentConfigArgoCd

		if cfg.ArgoCd == nil {
			conf = cfg.ArgoCdConfigs.ArgoCdConfigurations[0]
		} else {
			conf = cfg.ArgoCd
		}

		err := r.processApp(ctx, transaction, state, env, nil, conf, false, ts, fsMutex)

		if err != nil {
			return err
		}
	}
	return nil
}

func (r *repository) processApp(
	ctx context.Context,
	transaction *sql.Tx,
	state *State,
	env types.EnvName,
	commonEnvPrefix *string,
	currentArgoCdConfiguration *config.EnvironmentConfigArgoCd,
	isAAEnv bool,
	ts time.Time,
	fsMutex *sync.Mutex,
) error {
	prefix := ""
	if commonEnvPrefix != nil {
		prefix = *commonEnvPrefix
	}
	environmentInfo := &argocd.EnvironmentInfo{
		ArgoCDConfig:          currentArgoCdConfiguration,
		CommonPrefix:          prefix,
		ParentEnvironmentName: env,
		IsAAEnv:               isAAEnv,
	}
	projectNameOverride := types.ArgoProjectName("")
	if r.ArgoProjectNames != nil && r.ArgoProjectNames.ActiveActiveEnvironments != nil && r.ArgoProjectNames.Environments != nil {
		ok := false
		if isAAEnv {
			projectNameOverride, ok = (*r.ArgoProjectNames.ActiveActiveEnvironments)[types.EnvName(environmentInfo.GetFullyQualifiedName())]
		} else {
			projectNameOverride, ok = (*r.ArgoProjectNames.Environments)[types.EnvName(environmentInfo.GetFullyQualifiedName())]
		}
		if ok {
			environmentInfo.ArgoProjectNameOverride = projectNameOverride
		} else {
			environmentInfo.ArgoProjectNameOverride = ""
		}
	}
	err := r.processArgoAppForEnv(ctx, transaction, state, environmentInfo, ts, fsMutex)
	return err
}

func (r *repository) processArgoAppForEnv(ctx context.Context, transaction *sql.Tx, state *State, info *argocd.EnvironmentInfo, timestamp time.Time, fsMutex *sync.Mutex) error {
	_, appTeams, err := state.DBHandler.DBSelectEnvironmentApplicationsAtTimestamp(ctx, transaction, info.ParentEnvironmentName, timestamp)
	if err != nil {
		return err
	}
	spanCollectData, ctx := tracer.StartSpanFromContext(ctx, "collectData")
	defer spanCollectData.Finish()
	appData := []argocd.AppData{}
	deploymentsPerApp, err := db_history.DBSelectAppsWithDeploymentInEnvAtTimestamp(ctx, state.DBHandler, transaction, info.ParentEnvironmentName, timestamp)
	if err != nil {
		return err
	}
	for _, appWithTeam := range appTeams {
		appName := appWithTeam.AppName
		teamName := appWithTeam.TeamName
		deployment, ok := deploymentsPerApp[appName]
		if !ok {
			// nothing was deployed here at that time, skip:
			continue
		}
		if deployment.ReleaseNumbers.Version == nil || *deployment.ReleaseNumbers.Version == 0 {
			// There was a deployment here previously, but at the timestamp, nothing is deployed, skip:
			continue
		}
		appData = append(appData, argocd.AppData{
			AppName:  string(appName),
			TeamName: teamName,
		})
	}
	spanCollectData.Finish()

	spanRenderAndWrite, ctx := tracer.StartSpanFromContext(ctx, "RenderAndWrite")
	defer spanRenderAndWrite.Finish()
	manifests, err := argocd.Render(ctx, r.config.URL, r.config.Branch, info, appData)
	if err != nil {
		return err
	}
	return writeArgoCdManifestsSynced(ctx, state.Filesystem, info, manifests, fsMutex)
}

func writeArgoCdManifestsSynced(ctx context.Context, fs billy.Filesystem, info *argocd.EnvironmentInfo, manifests map[argocd.ApiVersion][]byte, fsMutex *sync.Mutex) error {
	span, _, _ := tracing.StartSpanFromContext(ctx, "writeArgoCdManifestsSynced") // We have a separate span here to see how long we wait for the mutex
	defer span.Finish()
	fsMutex.Lock()
	defer fsMutex.Unlock()
	return writeArgoCdManifests(ctx, fs, info, manifests)
}

func writeArgoCdManifests(ctx context.Context, fs billy.Filesystem, info *argocd.EnvironmentInfo, manifests map[argocd.ApiVersion][]byte) error {
	span, _, onErr := tracing.StartSpanFromContext(ctx, "writeArgoCdManifests")
	defer span.Finish()
	for apiVersion, content := range manifests {
		if err := fs.MkdirAll(fs.Join("argocd", string(apiVersion)), 0777); err != nil {
			return onErr(err)
		}
		target := getArgoCdAAEnvFileName(fs, types.EnvName(info.CommonPrefix), info.ParentEnvironmentName, types.EnvName(info.ArgoCDConfig.ConcreteEnvName), info.IsAAEnv)
		if err := util.WriteFile(fs, target, content, 0666); err != nil {
			return onErr(err)
		}
	}
	return nil
}

func getArgoCdAAEnvFileName(fs billy.Filesystem, commonEnvPrefix, parentEnvironmentName, concreteEnvironmentName types.EnvName, isAAEnv bool) string {
	if !isAAEnv {
		return fs.Join("argocd", string(argocd.V1Alpha1), fmt.Sprintf("%s.yaml", string(parentEnvironmentName)))
	}
	return fs.Join("argocd", string(argocd.V1Alpha1), fmt.Sprintf("%s.yaml", string(commonEnvPrefix)+"-"+string(parentEnvironmentName)+"-"+string(concreteEnvironmentName)))
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

func (s *State) GetAppsAndTeams() (map[types.AppName]string, error) {
	result, err := s.GetApplicationsFromFile()
	if err != nil {
		return nil, fmt.Errorf("could not get apps from file: %v", err)
	}
	var teamByAppName = map[types.AppName]string{} // key: app, value: team
	for i := range result {
		app := types.AppName(result[i])

		team, err := s.GetTeamNameFromManifest(string(app))
		if err != nil {
			// some apps do not have teams, that's not an error
			teamByAppName[app] = ""
		} else {
			teamByAppName[app] = team
		}
	}
	return teamByAppName, nil
}

func (s *State) GetTeamNameFromManifest(application string) (string, error) {
	fileSys := s.Filesystem

	teamFilePath := fileSys.Join("applications", application, "team")

	if teamName, err := util.ReadFile(fileSys, teamFilePath); err != nil {
		return "", err
	} else {
		return string(teamName), nil
	}
}

func decodeJsonFile(fs billy.Filesystem, path string, out interface{}) error {
	if file, err := fs.Open(path); err != nil {
		return wrapFileError(err, path, "could not decode json file")
	} else {
		defer func() { _ = file.Close() }()
		dec := json.NewDecoder(file)
		return dec.Decode(out)
	}
}

func (s *State) GetEnvironmentConfigFromManifest(environmentName string) (*config.EnvironmentConfig, error) {
	fileName := s.Filesystem.Join("environments", environmentName, "config.json")
	var envConfig config.EnvironmentConfig
	if err := decodeJsonFile(s.Filesystem, fileName, &envConfig); err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return nil, fmt.Errorf("%s : %w", fileName, ErrInvalidJson)
		}
	}
	return &envConfig, nil
}

func (s *State) GetAllEnvironmentConfigsFromManifest() (map[types.EnvName]config.EnvironmentConfig, error) {
	envs, err := s.Filesystem.ReadDir("environments")
	if err != nil {
		return nil, err
	}
	result := map[types.EnvName]config.EnvironmentConfig{}
	for _, env := range envs {
		c, err := s.GetEnvironmentConfigFromManifest(env.Name())
		if err != nil {
			return nil, err

		}
		result[types.EnvName(env.Name())] = *c
	}
	return result, nil
}
func (s *State) GetEnvironmentConfigsSortedFromManifest() (map[types.EnvName]config.EnvironmentConfig, []types.EnvName, error) {
	configs, err := s.GetAllEnvironmentConfigsFromManifest()
	if err != nil {
		return nil, nil, err
	}
	// sorting the environments to get a deterministic order of events:
	var envNames []types.EnvName = nil
	for envName := range configs {
		envNames = append(envNames, envName)
	}
	types.Sort(envNames)
	return configs, envNames, nil
}

// WriteCurrentlyDeployed writes all apps that have current deployments on any env from the filesystem to the database
func (s *State) WriteCurrentlyDeployed(ctx context.Context, transaction *sql.Tx, dbHandler *db.DBHandler) error {
	ddSpan, ctx := tracer.StartSpanFromContext(ctx, "WriteCurrentlyDeployed")
	defer ddSpan.Finish()
	_, envNames, err := s.GetEnvironmentConfigsSortedFromManifest() // this is intentional, when doing custom migrations (which is where this function is called), we want to read from the manifest repo explicitly
	if err != nil {
		return err
	}
	apps, err := s.GetApplicationsFromFile()
	if err != nil {
		return err
	}

	for _, appName := range apps {
		deploymentsForApp := map[types.EnvName]types.ReleaseNumbers{}
		for _, envName := range envNames {
			var version types.ReleaseNumbers
			version, err = s.GetEnvironmentApplicationVersionFromManifest(envName, appName)
			if err != nil {
				return fmt.Errorf("could not get version of app %s in env %s", appName, envName)
			}

			deploymentsForApp[envName] = version

			deployment := db.Deployment{
				Created:        time.Time{},
				App:            types.AppName(appName),
				Env:            envName,
				ReleaseNumbers: version,
				TransformerID:  0,
				Metadata: db.DeploymentMetadata{
					DeployedByName:  "",
					DeployedByEmail: "",
					CiLink:          "",
				},
			}
			err = dbHandler.DBUpdateOrCreateDeployment(ctx, transaction, deployment)
			if err != nil {
				return fmt.Errorf("error writing Deployment to DB for app %s in env %s: %w", deployment.App, deployment.Env, err)
			}
		}
	}
	return nil
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

func (s *State) WriteAllReleases(ctx context.Context, transaction *sql.Tx, app types.AppName, dbHandler *db.DBHandler) error {
	releases, err := s.GetAllApplicationReleasesFromManifest(string(app))
	if err != nil {
		return fmt.Errorf("cannot get releases of app %s: %v", app, err)
	}
	for i := range releases {
		releaseVersion := releases[i]
		repoRelease, err := s.GetApplicationReleaseFromManifest(string(app), releaseVersion)
		if err != nil {
			return fmt.Errorf("cannot get app release of app %s and release %v: %v", app, releaseVersion, err)
		}
		manifests, err := s.GetApplicationReleaseManifestsFromManifest(string(app), releaseVersion)
		if err != nil {
			return fmt.Errorf("cannot get manifest for app %s and release %v: %v", app, releaseVersion, err)
		}

		if !valid.SHA1CommitID(repoRelease.SourceCommitId) {
			//If we are about to import an invalid commit ID, simply log it and write an empty commit.
			logger.FromContext(ctx).Sugar().Warnf("Source commit ID %s is not valid. Skipping migration for release %d of app %s", repoRelease.SourceCommitId, releaseVersion, app)
			repoRelease.SourceCommitId = ""
		}

		var manifestsMap = map[types.EnvName]string{}
		for index := range manifests {
			manifest := manifests[index]
			manifestsMap[types.EnvName(manifest.Environment)] = manifest.Content
		}

		now, err := dbHandler.DBReadTransactionTimestamp(ctx, transaction)
		if err != nil {
			return fmt.Errorf("could not get transaction timestamp %v", err)

		}
		dbRelease := db.DBReleaseWithMetaData{
			Created: *now,
			ReleaseNumbers: types.ReleaseNumbers{
				Version:  &repoRelease.Version,
				Revision: repoRelease.Revision,
			},
			App: app,
			Manifests: db.DBReleaseManifests{
				Manifests: manifestsMap,
			},
			Metadata: db.DBReleaseMetaData{
				UndeployVersion: repoRelease.UndeployVersion,
				SourceAuthor:    repoRelease.SourceAuthor,
				SourceCommitId:  repoRelease.SourceCommitId,
				SourceMessage:   repoRelease.SourceMessage,
				DisplayVersion:  repoRelease.DisplayVersion,
				IsMinor:         false,
				IsPrepublish:    false,
				CiLink:          "",
			},
			Environments: []types.EnvName{},
		}
		err = dbHandler.DBUpdateOrCreateRelease(ctx, transaction, dbRelease)
		if err != nil {
			return fmt.Errorf("error writing Release to DB for app %s: %v", app, err)
		}
	}
	return nil
}

func (s *State) FixReleasesTimestamp(ctx context.Context, transaction *sql.Tx, app types.AppName, dbHandler *db.DBHandler) error {
	releases, err := s.GetAllApplicationReleasesFromManifest(string(app))
	if err != nil {
		return fmt.Errorf("cannot get releases of app %s: %v", app, err)
	}
	for i := range releases {
		releaseVersion := releases[i]
		repoRelease, err := s.GetApplicationReleaseFromManifest(string(app), releaseVersion)
		if err != nil {
			return fmt.Errorf("cannot get app release of app %s and release %v: %v", app, releaseVersion, err)
		}

		err = dbHandler.DBMigrationUpdateReleasesTimestamp(ctx, transaction, app, releaseVersion, repoRelease.CreatedAt)
		if err != nil {
			return fmt.Errorf("error writing Release to DB for app %s: %v", app, err)
		}
		envs, err := s.GetAllEnvironmentConfigsFromDB(ctx, transaction)
		if err != nil {
			return fmt.Errorf("error getting all envs: %v", err)
		}
		for env := range envs {
			logger.FromContext(ctx).Info(fmt.Sprintf("updating timestamp for %s, %s, %s", app, env, releaseVersion))
			_, createdAt, err := s.GetDeploymentMetaData(env, string(app))
			if err != nil {
				return fmt.Errorf("error getting deployment metadata: %v", err)
			}
			if !createdAt.IsZero() {
				err = dbHandler.DBMigrationUpdateDeploymentsTimestamp(ctx, transaction, app, repoRelease.Version, env, createdAt, repoRelease.Revision)
				if err != nil {
					return fmt.Errorf("error writing Deployment to DB for app %s and env %s: %v", app, env, err)
				}
			}
		}
	}
	return nil
}

func (r *repository) FixCommitsTimestamp(ctx context.Context, state State) error {
	revwalk, err := r.repository.Walk()
	if err != nil {
		return fmt.Errorf("failed to create revwalk: %v", err)
	}
	branchName := r.config.Branch
	if branchName == "" {
		branchName = "master"
	}
	branchRef, err := r.repository.References.Lookup(fmt.Sprintf("refs/heads/%s", branchName))
	if err != nil {
		return fmt.Errorf("failed to get branch reference: %v", err)
	}

	// Push HEAD to revwalk
	err = revwalk.Push(branchRef.Target())
	if err != nil {
		return fmt.Errorf("failed to push HEAD to revwalk: %v", err)
	}
	dbHandler := state.DBHandler
	err = dbHandler.WithTransaction(ctx, false, func(ctx context.Context, transaction *sql.Tx) error {
		err = revwalk.Iterate(func(commit *git.Commit) bool {
			commit, err := r.repository.LookupCommit(commit.Id())
			if err != nil {
				logger.FromContext(ctx).Sugar().Errorf("failed to lookup commit %s: %v", commit.Id().String(), err)
				return true // continue
			}

			logger.FromContext(ctx).Sugar().Infof("Commit: %s, Time: %s\n", commit.Id().String(), time.Unix(commit.Committer().When.Unix(), 0))
			err = dbHandler.DBUpdateCommitTransactionTimestamp(ctx, transaction, commit.Id().String(), commit.Committer().When)
			if err != nil {
				logger.FromContext(ctx).Sugar().Errorf("failed to lookup commit %s: %v", commit.Id().String(), err)
				return true
			}

			return true
		})
		return err
	})
	if err != nil {
		return fmt.Errorf("failed during revwalk: %v", err)
	}
	return nil
}

func (s *State) GetApplicationReleaseManifestsFromManifest(application string, version types.ReleaseNumbers) (map[types.EnvName]*api.Manifest, error) {
	manifests := map[types.EnvName]*api.Manifest{}
	dir := manifestDirectoryWithReleasesVersion(s.Filesystem, application, version)

	entries, err := s.Filesystem.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("reading manifest directory: %w", err)
	}
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

		manifests[types.EnvName(entry.Name())] = &api.Manifest{
			Environment: entry.Name(),
			Content:     string(content),
		}
	}
	return manifests, nil
}

func (s *State) GetApplicationReleaseFromManifest(application string, version types.ReleaseNumbers) (*Release, error) {
	base, err := s.checkWhichVersionDirectoryExists(s.Filesystem, application, version)
	if err != nil {
		base = releasesDirectoryWithVersionWithoutRevision(s.Filesystem, application, strconv.Itoa(int(*version.Version)))
		_, err := s.Filesystem.Stat(base)
		if err != nil {
			return nil, wrapFileError(err, base, "could not call stat")
		}
	}
	release := Release{
		Version:         *version.Version,
		UndeployVersion: false,
		SourceAuthor:    "",
		SourceCommitId:  "",
		SourceMessage:   "",
		CreatedAt:       time.Time{},
		DisplayVersion:  "",
		IsMinor:         false,
		IsPrepublish:    false,
		Environments:    []string{},
		Revision:        version.Revision,
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
	isUndeploy, err := s.IsUndeployVersionFromManifest(application, version)
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

func (s *State) IsUndeployVersionFromManifest(application string, version types.ReleaseNumbers) (bool, error) {
	base, err := s.checkWhichVersionDirectoryExists(s.Filesystem, application, version)
	if err != nil {
		base = releasesDirectoryWithVersionWithoutRevision(s.Filesystem, application, strconv.Itoa(int(*version.Version)))
		_, err := s.Filesystem.Stat(base)
		if err != nil {
			return false, wrapFileError(err, base, "could not call stat")
		}
	}
	if _, err := readFile(s.Filesystem, s.Filesystem.Join(base, "undeploy")); err != nil {
		if !os.IsNotExist(err) {
			return false, err
		}
		return false, nil
	}
	return true, nil
}

func (s *State) GetAllApplicationReleasesFromManifest(application string) ([]types.ReleaseNumbers, error) {
	if ns, err := names(s.Filesystem, s.Filesystem.Join("applications", application, "releases")); err != nil {
		return nil, err
	} else {
		result := make([]types.ReleaseNumbers, 0, len(ns))
		for _, n := range ns {
			r, err := types.MakeReleaseNumberFromString(n)
			if err == nil {
				result = append(result, r)
			}

		}
		sort.Slice(result, func(i, j int) bool {
			return types.Greater(result[j], result[i])
		})
		return result, nil
	}
}

// WriteCurrentEnvironmentLocks gets all locks on any environment in manifest and writes them to the DB
func (s *State) WriteCurrentEnvironmentLocks(ctx context.Context, transaction *sql.Tx, dbHandler *db.DBHandler) error {
	ddSpan, ctx := tracer.StartSpanFromContext(ctx, "WriteCurrentEnvironmentLocks")
	defer ddSpan.Finish()
	_, envNames, err := s.GetEnvironmentConfigsSortedFromManifest() // this is intentional, when doing custom migrations (which is where this function is called), we want to read from the manifest repo explicitly
	if err != nil {
		return err
	}
	for envNameIndex := range envNames {
		envName := envNames[envNameIndex]
		ls, err := s.GetEnvironmentLocksFromManifest(envName)
		if err != nil {
			return err
		}
		for lockId, lock := range ls {
			currentEnv := db.EnvironmentLock{
				Env:     envName,
				LockID:  lockId,
				Created: time.Time{}, //Time of insertion in the database
				Metadata: db.LockMetadata{
					CreatedByName:     lock.CreatedBy.Name,
					CreatedByEmail:    lock.CreatedBy.Email,
					Message:           lock.Message,
					CiLink:            "",             //CI links are not written into the manifest
					CreatedAt:         lock.CreatedAt, //Actual creation date
					SuggestedLifeTime: "",
				},
			}
			err = dbHandler.DBWriteEnvironmentLock(ctx, transaction, currentEnv.LockID, currentEnv.Env, currentEnv.Metadata)
			if err != nil {
				return fmt.Errorf("error writing environment locks to DB for environment %s: %w",
					envName, err)
			}
		}
		if err != nil {
			return fmt.Errorf("error writing environment locks ids to DB for environment %s: %w",
				envName, err)
		}
	}
	return nil
}

func (s *State) GetEnvironmentLocksFromManifest(environment types.EnvName) (map[string]Lock, error) {
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

func (s *State) WriteCurrentApplicationLocks(ctx context.Context, transaction *sql.Tx, dbHandler *db.DBHandler) error {
	ddSpan, ctx := tracer.StartSpanFromContext(ctx, "WriteCurrentApplicationLocks")
	defer ddSpan.Finish()
	_, envNames, err := s.GetEnvironmentConfigsSortedFromManifest() // this is intentional, when doing custom migrations (which is where this function is called), we want to read from the manifest repo explicitly

	if err != nil {
		return err
	}
	for envNameIndex := range envNames {

		envName := envNames[envNameIndex]

		appNames, err := s.GetEnvironmentApplicationsFromManifest(envName)
		if err != nil {
			return err
		}

		for _, currentApp := range appNames {
			ls, err := s.GetEnvironmentApplicationLocksFromManifest(envName, currentApp)
			if err != nil {
				return err
			}
			for lockId, lock := range ls {
				currentAppLock := db.ApplicationLock{
					Env:     envName,
					LockID:  lockId,
					Created: time.Time{},
					Metadata: db.LockMetadata{
						CreatedByName:     lock.CreatedBy.Name,
						CreatedByEmail:    lock.CreatedBy.Email,
						Message:           lock.Message,
						CiLink:            "", //CI links are not written into the manifest
						CreatedAt:         lock.CreatedAt,
						SuggestedLifeTime: "",
					},
					App: types.AppName(currentApp),
				}
				err = dbHandler.DBWriteApplicationLock(ctx, transaction, currentAppLock.LockID, currentAppLock.Env, currentAppLock.App, currentAppLock.Metadata)
				if err != nil {
					return fmt.Errorf("error writing application locks to DB for application '%s' on '%s': %w",
						currentApp, envName, err)
				}
			}
		}
	}
	return nil
}

func (s *State) GetEnvironmentApplicationsFromManifest(environment types.EnvName) ([]string, error) {
	appDir := s.Filesystem.Join("environments", string(environment), "applications")
	return names(s.Filesystem, appDir)
}

func (s *State) GetEnvironmentApplicationLocksFromManifest(environment types.EnvName, application string) (map[string]Lock, error) {
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

func (s *State) WriteCurrentTeamLocks(ctx context.Context, transaction *sql.Tx, dbHandler *db.DBHandler) error {
	ddSpan, _ := tracer.StartSpanFromContext(ctx, "WriteCurrentTeamLocks")
	defer ddSpan.Finish()
	_, envNames, err := s.GetEnvironmentConfigsSortedFromManifest() // this is intentional, when doing custom migrations (which is where this function is called), we want to read from the manifest repo explicitly

	if err != nil {
		return err
	}

	for envNameIndex := range envNames {
		processedTeams := map[string]bool{} //TeamName -> boolean (processed or not)
		envName := envNames[envNameIndex]

		appNames, err := s.GetEnvironmentApplicationsFromManifest(envName)
		if err != nil {
			return err
		}

		for _, currentApp := range appNames {

			teamName, err := s.GetTeamNameFromManifest(currentApp)
			if err != nil {
				if errors.Is(err, os.ErrNotExist) {
					continue //If app has no team, we skip it
				}
				return err
			}
			_, exists := processedTeams[teamName]
			if !exists {
				processedTeams[teamName] = true
			} else {
				continue
			}

			ls, err := s.GetEnvironmentTeamLocksFromManifest(envName, teamName)
			if err != nil {
				return err
			}
			for lockId, lock := range ls {
				currentTeamLock := db.TeamLock{
					Env:     envName,
					LockID:  lockId,
					Created: time.Time{},
					Metadata: db.LockMetadata{
						CreatedByName:     lock.CreatedBy.Name,
						CreatedByEmail:    lock.CreatedBy.Email,
						Message:           lock.Message,
						CiLink:            "", //CI links are not written into the manifest
						CreatedAt:         lock.CreatedAt,
						SuggestedLifeTime: "",
					},
					Team: teamName,
				}
				err = dbHandler.DBWriteTeamLock(ctx, transaction, currentTeamLock.LockID, currentTeamLock.Env, currentTeamLock.Team, currentTeamLock.Metadata)
				if err != nil {
					return fmt.Errorf("error writing team locks to DB for team '%s' on '%s': %w",
						teamName, envName, err)
				}
			}
		}
	}
	return nil
}

func (s *State) GetEnvironmentTeamLocksFromManifest(environment types.EnvName, team string) (map[string]Lock, error) {
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

// for use with custom migrations, otherwise use the two functions above
func (s *State) GetAllEnvironments(_ context.Context) (map[types.EnvName]config.EnvironmentConfig, error) {
	result := map[types.EnvName]config.EnvironmentConfig{}

	fileSys := s.Filesystem

	envDir, err := fileSys.ReadDir("environments")
	if err != nil {
		return nil, fmt.Errorf("error while reading the environments directory, error: %w", err)
	}

	for _, envName := range envDir {
		configFilePath := fileSys.Join("environments", envName.Name(), "config.json")
		configBytes, err := readFile(fileSys, configFilePath)
		if err != nil {
			return nil, fmt.Errorf("could not read file at %s, error: %w", configFilePath, err)
		}
		//exhaustruct:ignore
		cfg := config.EnvironmentConfig{}
		err = json.Unmarshal(configBytes, &cfg)
		if err != nil {
			return nil, fmt.Errorf("error while unmarshaling the database JSON, error: %w", err)
		}
		result[types.EnvName(envName.Name())] = cfg
	}

	return result, nil
}

func (s *State) WriteAllQueuedAppVersions(ctx context.Context, transaction *sql.Tx, dbHandler *db.DBHandler) error {
	ddSpan, ctx := tracer.StartSpanFromContext(ctx, "GetAllQueuedAppVersions")
	defer ddSpan.Finish()
	_, envNames, err := s.GetEnvironmentConfigsSortedFromManifest()

	if err != nil {
		return err
	}
	for envNameIndex := range envNames {

		envName := envNames[envNameIndex]

		appNames, err := s.GetEnvironmentApplicationsFromManifest(envName)
		if err != nil {
			return err
		}

		for _, currentApp := range appNames {
			var version types.ReleaseNumbers
			version, err := s.GetQueuedVersionFromManifest(envName, currentApp)
			if err != nil {
				return err
			}

			err = dbHandler.DBWriteDeploymentAttempt(ctx, transaction, envName, types.AppName(currentApp), version)
			if err != nil {
				return fmt.Errorf("error writing existing queued application version '%v' to DB for app '%s' on environment '%s': %w",
					version, currentApp, envName, err)
			}
		}
	}
	return nil
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

func (s *State) GetEnvLocksDir(environment types.EnvName) string {
	return s.Filesystem.Join("environments", string(environment), "locks")
}

func (s *State) GetEnvLockDir(environment types.EnvName, lockId string) string {
	return s.Filesystem.Join(s.GetEnvLocksDir(environment), lockId)
}

func (s *State) GetAppLocksDir(environment types.EnvName, application string) string {
	return s.Filesystem.Join("environments", string(environment), "applications", application, "locks")
}
func (s *State) GetTeamLocksDir(environment types.EnvName, team string) string {
	return s.Filesystem.Join("environments", string(environment), "teams", team, "locks")
}

func (s *State) GetEnvironmentLocksFromDB(ctx context.Context, transaction *sql.Tx, environment types.EnvName) (map[string]Lock, error) {
	lockIds, err := s.DBHandler.DBSelectAllEnvLocks(ctx, transaction, environment)
	if err != nil {
		return nil, err
	}

	locks, err := s.DBHandler.DBSelectEnvLockSet(ctx, transaction, environment, lockIds)

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

func (s *State) GetLastRelease(ctx context.Context, fs billy.Filesystem, application string) (types.ReleaseNumbers, error) {
	var err error
	releasesDir := releasesDirectory(fs, application)
	err = fs.MkdirAll(releasesDir, 0777)
	if err != nil {
		return types.MakeEmptyReleaseNumbers(), err
	}
	if entries, err := fs.ReadDir(releasesDir); err != nil {
		return types.MakeEmptyReleaseNumbers(), err
	} else {
		var lastRelease types.ReleaseNumbers
		for _, e := range entries {
			if curr, err := types.MakeReleaseNumberFromString(e.Name()); err != nil {
				logger.FromContext(ctx).Sugar().Warnf("Bad name for release: '%s'\n", e.Name())
			} else {
				if lastRelease.Version == nil || types.Greater(curr, lastRelease) {
					lastRelease = curr
				}
			}
		}
		return lastRelease, nil
	}
}

func (s *State) GetEnvironmentApplicationLocksFromDB(ctx context.Context, transaction *sql.Tx, environment types.EnvName, application string) (map[string]Lock, error) {
	if transaction == nil {
		return nil, fmt.Errorf("GetEnvironmentApplicationLocksFromDB: No transaction provided")
	}
	lockIds, err := s.DBHandler.DBSelectAllAppLocks(ctx, transaction, environment, types.AppName(application))
	if err != nil {
		return nil, err
	}
	locks, err := s.DBHandler.DBSelectAppLockSet(ctx, transaction, environment, types.AppName(application), lockIds)

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

func (s *State) GetEnvironmentTeamLocksFromDB(ctx context.Context, transaction *sql.Tx, environment types.EnvName, team string) (map[string]Lock, error) {
	if team == "" {
		return map[string]Lock{}, nil
	}

	if transaction == nil {
		return nil, fmt.Errorf("GetEnvironmentTeamLocksFromDB: No transaction provided")
	}
	lockIds, err := s.DBHandler.DBSelectAllTeamLocks(ctx, transaction, environment, team)
	if err != nil {
		return nil, err
	}

	locks, err := s.DBHandler.DBSelectTeamLockSet(ctx, transaction, environment, team, lockIds)

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

func (s *State) GetDeploymentMetaData(environment types.EnvName, application string) (string, time.Time, error) {
	base := s.Filesystem.Join("environments", string(environment), "applications", application)
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

func (s *State) DeleteTeamLockIfEmpty(ctx context.Context, environment types.EnvName, team string) error {
	dir := s.GetTeamLocksDir(environment, team)
	_, err := s.DeleteDirIfEmpty(dir)
	return err
}

func (s *State) DeleteAppLockIfEmpty(ctx context.Context, environment types.EnvName, application string) error {
	dir := s.GetAppLocksDir(environment, application)
	_, err := s.DeleteDirIfEmpty(dir)
	return err
}

func (s *State) DeleteEnvLockIfEmpty(ctx context.Context, environment types.EnvName) error {
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

func (s *State) GetQueuedVersionFromManifest(environment types.EnvName, application string) (types.ReleaseNumbers, error) {
	return s.readSymlink(environment, application, queueFileName)
}

func (s *State) DeleteQueuedVersion(environment types.EnvName, application string) error {
	queuedVersion := s.Filesystem.Join("environments", string(environment), "applications", application, queueFileName)
	return s.Filesystem.Remove(queuedVersion)
}

func (s *State) DeleteQueuedVersionIfExists(environment types.EnvName, application string) error {
	queuedVersion, err := s.GetQueuedVersionFromManifest(environment, application)
	if err != nil {
		return err
	}
	if queuedVersion.Version == nil {
		return nil // nothing to do
	}
	return s.DeleteQueuedVersion(environment, application)
}

func (s *State) GetEnvironmentApplicationVersion(ctx context.Context, transaction *sql.Tx, environment types.EnvName, application string) (types.ReleaseNumbers, error) {
	depl, err := s.DBHandler.DBSelectLatestDeployment(ctx, transaction, types.AppName(application), environment)
	if err != nil {
		return types.MakeEmptyReleaseNumbers(), err
	}
	if depl == nil || depl.ReleaseNumbers.Version == nil {
		return types.MakeEmptyReleaseNumbers(), nil
	}

	return depl.ReleaseNumbers, nil
}

func (s *State) GetEnvironmentApplicationVersionAtTimestamp(ctx context.Context, transaction *sql.Tx, environment types.EnvName, application string, ts time.Time) (types.ReleaseNumbers, error) {
	depl, err := s.DBHandler.DBSelectLatestDeploymentAtTimestamp(ctx, transaction, types.AppName(application), environment, ts)
	if err != nil {
		return types.MakeEmptyReleaseNumbers(), err
	}
	if depl == nil || depl.ReleaseNumbers.Version == nil {
		return types.MakeEmptyReleaseNumbers(), nil
	}

	return depl.ReleaseNumbers, nil
}

func (s *State) GetEnvironmentApplicationVersionFromManifest(environment types.EnvName, application string) (types.ReleaseNumbers, error) {
	return s.readSymlink(environment, application, "version")
}

// returns nil if there is no file
func (s *State) readSymlink(environment types.EnvName, application string, symlinkName string) (types.ReleaseNumbers, error) {
	version := s.Filesystem.Join("environments", string(environment), "applications", application, symlinkName)
	if lnk, err := s.Filesystem.Readlink(version); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			// if the link does not exist, we return nil
			return types.MakeEmptyReleaseNumbers(), nil
		}
		return types.MakeEmptyReleaseNumbers(), fmt.Errorf("failed reading symlink %q: %w", version, err)
	} else {
		target := s.Filesystem.Join("environments", string(environment), "applications", application, lnk)
		if stat, err := s.Filesystem.Stat(target); err != nil {
			// if the file that the link points to does not exist, that's an error
			return types.MakeEmptyReleaseNumbers(), fmt.Errorf("failed stating %q: %w", target, err)
		} else {
			res, err := types.MakeReleaseNumberFromString(stat.Name())
			return res, err
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

var ErrInvalidJson = errors.New("JSON file is not valid")

func envExists(envConfigs map[types.EnvName]config.EnvironmentConfig, envNameToSearchFor types.EnvName) bool {
	if _, found := envConfigs[envNameToSearchFor]; found {
		return true
	}
	return false
}

func (s *State) GetAllEnvironmentConfigsFromDB(ctx context.Context, transaction *sql.Tx) (map[types.EnvName]config.EnvironmentConfig, error) {
	dbAllEnvs, err := s.DBHandler.DBSelectAllEnvironments(ctx, transaction)
	if err != nil {
		return nil, fmt.Errorf("unable to retrieve all environments, error: %w", err)
	}
	if dbAllEnvs == nil {
		return nil, nil
	}
	envs, err := s.DBHandler.DBSelectEnvironmentsBatch(ctx, transaction, dbAllEnvs)
	if err != nil {
		return nil, fmt.Errorf("unable to retrieve manifests for environments %v from the database, error: %w", dbAllEnvs, err)
	}
	ret := make(map[types.EnvName]config.EnvironmentConfig)
	for _, env := range *envs {
		ret[env.Name] = env.Config
	}
	return ret, nil
}

func (s *State) GetAllEnvironmentConfigsFromDBAtTimestamp(ctx context.Context, transaction *sql.Tx, ts time.Time) (map[types.EnvName]config.EnvironmentConfig, error) {
	envs, err := s.DBHandler.DBSelectAllLatestEnvironmentsAtTimestamp(ctx, transaction, ts)
	if err != nil {
		return nil, fmt.Errorf("unable to retrieve environment configurations from the database, error: %w", err)
	}
	ret := make(map[types.EnvName]config.EnvironmentConfig)
	for _, env := range *envs {
		ret[env.Name] = env.Config
	}
	return ret, nil
}

func (s *State) GetEnvironmentConfigsAndValidate(ctx context.Context, transaction *sql.Tx) (map[types.EnvName]config.EnvironmentConfig, error) {
	logger := logger.FromContext(ctx)
	envConfigs, err := s.GetAllEnvironmentConfigsFromManifest()
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

func (s *State) GetEnvironmentConfigsForGroup(ctx context.Context, transaction *sql.Tx, envGroup string) ([]types.EnvName, error) {
	allEnvConfigs, err := s.GetAllEnvironmentConfigsFromDB(ctx, transaction)
	if err != nil {
		return nil, err
	}
	groupEnvNames := []types.EnvName{}
	for env := range allEnvConfigs {
		envConfig := allEnvConfigs[env]
		g := envConfig.EnvironmentGroup
		if g != nil && *g == envGroup {
			groupEnvNames = append(groupEnvNames, env)
		}
	}
	if len(groupEnvNames) == 0 {
		return nil, fmt.Errorf("no environment found with given group '%s'", envGroup)
	}
	types.Sort(groupEnvNames)
	return groupEnvNames, nil
}

func (s *State) GetEnvironmentApplications(ctx context.Context, transaction *sql.Tx, environment types.EnvName) ([]types.AppName, error) {
	envApps, err := s.DBHandler.DBSelectEnvironmentApplications(ctx, transaction, environment)
	if err != nil {
		return make([]types.AppName, 0), err
	}
	if envApps == nil {
		return make([]types.AppName, 0), nil
	}
	return envApps, nil
}

// GetApplicationsFromFile returns apps from the filesystem
func (s *State) GetApplicationsFromFile() ([]string, error) {
	return names(s.Filesystem, "applications")
}

func (s *State) GetApplicationReleasesFromFile(application string) ([]types.ReleaseNumbers, error) {
	if ns, err := names(s.Filesystem, s.Filesystem.Join("applications", application, "releases")); err != nil {
		return nil, err
	} else {
		result := make([]types.ReleaseNumbers, 0, len(ns))
		for _, n := range ns {
			if i, err := strconv.ParseUint(n, 10, 64); err == nil {
				result = append(result, types.MakeReleaseNumberVersion(i))
			} else {
				if ver, err := types.MakeReleaseNumberFromString(n); err == nil {
					result = append(result, ver)
				}
			}
		}
		sort.Slice(result, func(i, j int) bool {
			return types.Greater(result[j], result[i])
		})
		return result, nil
	}
}

type Release struct {
	Version  uint64
	Revision uint64
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
	IsMinor         bool
	/**
	"IsPrepublish=true" is used at the start of the merge pipeline to create a pre-publish release which can't be deployed.
	The goal is to get 100% of the commits even if the pipeline fails.
	*/
	IsPrepublish bool
	Environments []string
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
		Environments:    []string{},
		CiLink:          "", //does not matter here
		Revision:        rel.Revision,
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

func (s *State) IsUndeployVersion(application string, version types.ReleaseNumbers) (bool, error) {
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

func (s *State) GetApplicationRelease(application string, version types.ReleaseNumbers) (*Release, error) {
	base := releasesDirectoryWithVersion(s.Filesystem, application, version)
	_, err := s.Filesystem.Stat(base)
	if err != nil {
		return nil, wrapFileError(err, base, "could not call stat")
	}
	release := Release{
		Version:         *version.Version,
		UndeployVersion: false,
		SourceAuthor:    "",
		SourceCommitId:  "",
		SourceMessage:   "",
		CreatedAt:       time.Time{},
		DisplayVersion:  "",
		IsMinor:         false,
		IsPrepublish:    false,
		Environments:    nil,
		Revision:        version.Revision,
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

func (s *State) GetApplicationReleaseManifests(application string, version types.ReleaseNumbers) (map[string]*api.Manifest, error) {
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
	app, err := s.DBHandler.DBSelectApp(ctx, transaction, types.AppName(application))
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

func readFile(fs billy.Filesystem, path string) ([]byte, error) {
	if file, err := fs.Open(path); err != nil {
		return nil, err
	} else {
		defer func() { _ = file.Close() }()
		return io.ReadAll(file)
	}
}

// ProcessQueue checks if there is something in the queue
// deploys if necessary
// deletes the queue
func (s *State) ProcessQueue(ctx context.Context, transaction *sql.Tx, fs billy.Filesystem, environment types.EnvName, application string) (string, error) {
	queuedVersion, err := s.GetQueuedVersionFromManifest(environment, application)
	queueDeploymentMessage := ""
	if err != nil {
		// could not read queued version.
		return "", err
	} else {
		if queuedVersion.Version == nil {
			// if there is no version queued, that's not an issue, just do nothing:
			return "", nil
		}

		currentlyDeployedVersion, err := s.GetEnvironmentApplicationVersion(ctx, transaction, environment, application)
		if err != nil {
			return "", err
		}

		if currentlyDeployedVersion.Version != nil && types.Equal(queuedVersion, currentlyDeployedVersion) {
			// delete queue, it's outdated! But if we can't, that's not really a problem, as it would be overwritten
			// whenever the next deployment happens:
			err = s.DeleteQueuedVersion(environment, application)
			return fmt.Sprintf("deleted queued version %v because it was already deployed. app=%q env=%q", queuedVersion, application, environment), err
		}
	}
	return queueDeploymentMessage, nil
}

func GetTags(ctx context.Context, handler *db.DBHandler, cfg RepositoryConfig, repoName string) (tags []*api.TagData, err error) {
	span, ctx := tracer.StartSpanFromContext(ctx, "getTags")
	defer func() { span.Finish(tracer.WithError(err)) }()
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

	fetchSpan, _ := tracer.StartSpanFromContext(ctx, "getTags-FetchingRemote")
	err = remote.Fetch([]string{fetchSpec}, &fetchOptions, "fetching")
	if err != nil {
		fetchSpan.Finish(tracer.WithError(err))
		return nil, fmt.Errorf("failure to fetch: %v", err)
	}
	fetchSpan.Finish()

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
		loopSpan, ctxLoop := tracer.StartSpanFromContext(ctx, "getTags-for")
		tagObject, err := iters.Next()
		if err != nil {
			loopSpan.Finish(tracer.WithError(err))
			break
		}
		tagRef, lookupErr := repo.LookupTag(tagObject.Target())
		var tag *api.TagData
		var tagName string
		var commitId string
		if lookupErr != nil {
			tagCommit, err := repo.LookupCommit(tagObject.Target())
			// If LookupTag fails, fallback to LookupCommit
			// to cover all tags, annotated and lightweight
			if err != nil {
				e := fmt.Errorf("unable to lookup tag [%s]: %v - original err: %v", tagObject.Name(), err, lookupErr)
				loopSpan.Finish(tracer.WithError(e))
				return nil, e
			}
			tagName = tagObject.Name()
			commitId = tagCommit.Id().String()
		} else {
			tagCommit, err := repo.LookupCommit(tagRef.TargetId())
			if err != nil {
				err = fmt.Errorf("unable to lookup tag [%s]: %v", tagObject.Name(), err)
				loopSpan.Finish(tracer.WithError(err))
				return nil, err
			}
			tagName = tagObject.Name()
			commitId = tagCommit.Id().String()
		}

		result, err := db.WithTransactionT(handler, ctxLoop, 2, true, func(ctx context.Context, transaction *sql.Tx) (*time.Time, error) {
			return handler.DBReadCommitHashTransactionTimestamp(ctx, transaction, commitId)
		})
		if err != nil {
			return nil, fmt.Errorf("withtransaction: %w", err)
		}

		tag = &api.TagData{
			Tag:        tagName,
			CommitId:   commitId,
			CommitDate: nil,
		}
		if result != nil {
			tag.CommitDate = timestamppb.New(*result)
		}
		// else: could not find a commit date - this means something went wrong before this endpoint was called
		// e.g. in a db migration
		tags = append(tags, tag)
		loopSpan.Finish()
	}

	return tags, nil
}

func (r *repository) Notify() *notify.Notify {
	return &r.notify
}

func MeasureGitSyncStatus(ctx context.Context, ddMetrics statsd.ClientInterface, dbHandler *db.DBHandler) (err error) {
	if ddMetrics != nil {
		span, ctx := tracer.StartSpanFromContext(ctx, "MeasureGitSyncStatus")
		defer func() { span.Finish(tracer.WithError(err)) }()
		var results *[2]int
		results, err = db.WithTransactionT[[2]int](dbHandler, ctx, 2, true, func(ctx context.Context, transaction *sql.Tx) (*[2]int, error) {
			unsyncedStatuses, err := dbHandler.DBRetrieveAppsByStatus(ctx, transaction, db.UNSYNCED)
			if err != nil {
				return &[2]int{}, err
			}

			syncFailedStatuses, err := dbHandler.DBRetrieveAppsByStatus(ctx, transaction, db.SYNC_FAILED)
			if err != nil {
				return &[2]int{}, err
			}

			return &[2]int{len(unsyncedStatuses), len(syncFailedStatuses)}, nil
		})

		if err != nil {
			return err
		}

		if err = ddMetrics.Gauge("git_sync_unsynced", float64(results[0]), []string{}, 1); err != nil {
			return err
		}

		if err = ddMetrics.Gauge("git_sync_failed", float64(results[1]), []string{}, 1); err != nil {
			return err
		}
	}

	return nil
}
