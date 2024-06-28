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
	"bufio"
	"bytes"
	"context"
	"crypto/tls"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/freiheit-com/kuberpult/pkg/event"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/freiheit-com/kuberpult/pkg/conversion"
	time2 "github.com/freiheit-com/kuberpult/pkg/time"

	"github.com/freiheit-com/kuberpult/pkg/argocd"
	"github.com/freiheit-com/kuberpult/pkg/argocd/v1alpha1"
	"github.com/freiheit-com/kuberpult/pkg/db"
	"github.com/freiheit-com/kuberpult/pkg/mapper"

	"github.com/freiheit-com/kuberpult/pkg/grpc"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/DataDog/datadog-go/v5/statsd"
	backoff "github.com/cenkalti/backoff/v4"
	api "github.com/freiheit-com/kuberpult/pkg/api/v1"
	"github.com/freiheit-com/kuberpult/pkg/auth"
	"github.com/freiheit-com/kuberpult/pkg/config"
	"github.com/freiheit-com/kuberpult/pkg/setup"
	"github.com/freiheit-com/kuberpult/services/cd-service/pkg/cloudrun"
	"github.com/freiheit-com/kuberpult/services/cd-service/pkg/fs"
	"github.com/freiheit-com/kuberpult/services/cd-service/pkg/notify"
	"github.com/freiheit-com/kuberpult/services/cd-service/pkg/sqlitestore"
	"go.uber.org/zap"
	"gopkg.in/DataDog/dd-trace-go.v1/ddtrace/tracer"

	"github.com/freiheit-com/kuberpult/pkg/logger"
	billy "github.com/go-git/go-billy/v5"
	"github.com/go-git/go-billy/v5/util"
	git "github.com/libgit2/git2go/v34"
)

type contextKey string

const DdMetricsKey contextKey = "ddMetrics"

// A Repository provides a multiple reader / single writer access to a git repository.
type Repository interface {
	Apply(ctx context.Context, transformers ...Transformer) error
	Push(ctx context.Context, pushAction func() error) error
	ApplyTransformersInternal(ctx context.Context, transaction *sql.Tx, transformers ...Transformer) ([]string, *State, []*TransformerResult, *TransformerBatchApplyError)
	State() *State
	StateAt(oid *git.Oid) (*State, error)
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

var (
	ddMetrics statsd.ClientInterface
)

type StorageBackend int

const (
	DefaultBackend StorageBackend = 0
	GitBackend     StorageBackend = iota
	SqliteBackend  StorageBackend = iota
)

const (
	maxArgoRequests = 3 // note that this happens inside a request, we cannot retry too much!
)

type repository struct {
	// Mutex gurading the writer
	writeLock    sync.Mutex
	writesDone   uint
	queue        queue
	config       *RepositoryConfig
	credentials  *credentialsStore
	certificates *certificateStore

	repository *git.Repository

	// Mutex guarding head
	headLock sync.Mutex

	notify notify.Notify

	backOffProvider func() backoff.BackOff

	DB *db.DBHandler
}

type WebhookResolver interface {
	Resolve(insecure bool, req *http.Request) (*http.Response, error)
}

type DefaultWebhookResolver struct{}

func (r DefaultWebhookResolver) Resolve(insecure bool, req *http.Request) (*http.Response, error) {
	//exhaustruct:ignore
	TLSClientConfig := &tls.Config{
		InsecureSkipVerify: insecure,
	}
	//exhaustruct:ignore
	tr := &http.Transport{
		TLSClientConfig: TLSClientConfig,
	}
	//exhaustruct:ignore
	client := &http.Client{
		Transport: tr,
	}
	return client.Do(req)
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
	//
	GcFrequency uint
	// number of app versions to keep a history of
	ReleaseVersionsLimit uint
	StorageBackend       StorageBackend
	// Bootstrap mode controls where configurations are read from
	// true: read from json file at EnvironmentConfigsPath
	// false: read from config files in manifest repo
	BootstrapMode          bool
	EnvironmentConfigsPath string
	ArgoInsecure           bool
	// if set, kuberpult will generate push events to argoCd whenever it writes to the manifest repo:
	ArgoWebhookUrl string
	// the url to the git repo, like the browser requires it (https protocol)
	WebURL                string
	DogstatsdEvents       bool
	WriteCommitData       bool
	WebhookResolver       WebhookResolver
	MaximumCommitsPerPush uint
	MaximumQueueSize      uint
	// Extend maximum AppName length
	AllowLongAppNames bool

	ArgoCdGenerateFiles bool

	DBHandler      *db.DBHandler
	CloudRunClient *cloudrun.CloudRunClient
}

func openOrCreate(path string, storageBackend StorageBackend) (*git.Repository, error) {
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
	if storageBackend == SqliteBackend {
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
	}
	return repo2, err
}

func GetTags(cfg RepositoryConfig, repoName string, ctx context.Context) (tags []*api.TagData, err error) {
	repo, err := openOrCreate(repoName, cfg.StorageBackend)
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

// Opens a repository. The repository is initialized and updated in the background.
func New(ctx context.Context, cfg RepositoryConfig) (Repository, error) {
	repo, bg, err := New2(ctx, cfg)
	if err != nil {
		return nil, err
	}
	go bg(ctx, nil) //nolint: errcheck
	return repo, err
}

func New2(ctx context.Context, cfg RepositoryConfig) (Repository, setup.BackgroundFunc, error) {
	logger := logger.FromContext(ctx)

	ddMetricsFromCtx := ctx.Value(DdMetricsKey)
	if ddMetricsFromCtx != nil {
		ddMetrics = ddMetricsFromCtx.(statsd.ClientInterface)
	} else {
		logger.Sugar().Warnf("could not load ddmetrics from context - running without datadog metrics")
	}

	if cfg.Branch == "" {
		cfg.Branch = "master"
	}
	if cfg.CommitterEmail == "" {
		cfg.CommitterEmail = "kuberpult@example.com"
	}
	if cfg.CommitterName == "" {
		cfg.CommitterName = "kuberpult"
	}
	if cfg.StorageBackend == DefaultBackend {
		cfg.StorageBackend = SqliteBackend
	}
	if cfg.NetworkTimeout == 0 {
		cfg.NetworkTimeout = time.Minute
	}
	if cfg.MaximumCommitsPerPush == 0 {
		cfg.MaximumCommitsPerPush = 1

	}
	if cfg.MaximumQueueSize == 0 {
		cfg.MaximumQueueSize = 5
	}
	// The value here is set to keptVersionsOnCleanup to maintain compatibility with tests that do not pass ReleaseVersionsLimit in the repository config
	if cfg.ReleaseVersionsLimit == 0 {
		cfg.ReleaseVersionsLimit = keptVersionsOnCleanup
	}

	var credentials *credentialsStore
	var certificates *certificateStore
	var err error
	if strings.HasPrefix(cfg.URL, "./") || strings.HasPrefix(cfg.URL, "/") {
		logger.Debug("git url indicates a local directory. Ignoring credentials and certificates.")
	} else {
		credentials, err = cfg.Credentials.load()
		if err != nil {
			return nil, nil, err
		}
		certificates, err = cfg.Certificates.load()
		if err != nil {
			return nil, nil, err
		}
	}

	if repo2, err := openOrCreate(cfg.Path, cfg.StorageBackend); err != nil {
		return nil, nil, err
	} else {
		// configure remotes
		if remote, err := repo2.Remotes.CreateAnonymous(cfg.URL); err != nil {
			return nil, nil, err
		} else {
			result := &repository{
				writesDone:      0,
				headLock:        sync.Mutex{},
				notify:          notify.Notify{},
				writeLock:       sync.Mutex{},
				config:          &cfg,
				credentials:     credentials,
				certificates:    certificates,
				repository:      repo2,
				queue:           makeQueueN(cfg.MaximumQueueSize),
				backOffProvider: defaultBackOffProvider,
				DB:              cfg.DBHandler,
			}
			result.headLock.Lock()

			defer result.headLock.Unlock()
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
				return nil, nil, err
			}
			var rev *git.Oid
			if remoteRef, err := repo2.References.Lookup(fmt.Sprintf("refs/remotes/origin/%s", cfg.Branch)); err != nil {
				var gerr *git.GitError
				if errors.As(err, &gerr) && gerr.Code == git.ErrorCodeNotFound {
					// not found
					// nothing to do
				} else {
					return nil, nil, err
				}
			} else {
				rev = remoteRef.Target()
				if _, err := repo2.References.Create(fmt.Sprintf("refs/heads/%s", cfg.Branch), rev, true, "reset branch"); err != nil {
					return nil, nil, err
				}
			}

			// check that we can build the current state
			state, err := result.StateAt(nil)
			if err != nil {
				return nil, nil, err
			}

			// Check configuration for errors and abort early if any:
			if state.DBHandler.ShouldUseOtherTables() {
				_, err = db.WithTransactionT(state.DBHandler, ctx, func(ctx context.Context, transaction *sql.Tx) (*map[string]config.EnvironmentConfig, error) {
					ret, err := state.GetEnvironmentConfigsAndValidate(ctx, transaction)
					return &ret, err
				})
			} else {
				_, err = state.GetEnvironmentConfigsAndValidate(ctx, nil)
			}

			if err != nil {
				return nil, nil, err
			}

			return result, result.ProcessQueue, nil
		}
	}
}

func (r *repository) ProcessQueue(ctx context.Context, health *setup.HealthReporter) error {
	defer func() {
		close(r.queue.transformerBatches)
		for e := range r.queue.transformerBatches {
			e.finish(ctx.Err())
		}
	}()
	tick := time.Tick(r.config.NetworkTimeout) //nolint: staticcheck
	ttl := r.config.NetworkTimeout * 3
	for {
		/*
			One tricky issue is that `git push` can take a while depending on the git hoster and the connection
			(plus we do have relatively big and many commits).
			This can lead to the situation that "everything hangs", because there is one push running already -
			but only one push is possible at a time.
			There is also no good way to cancel a `git push`.

			To circumvent this, we report health with a "time to live" - meaning if we don't report anything within the time frame,
			the health will turn to "failed" and then the pod will automatically restart (in kubernetes).
		*/
		health.ReportHealthTtl(setup.HealthReady, "processing queue", &ttl)
		select {
		case <-tick:
			// this triggers a for loop every `NetworkTimeout` to refresh the readiness
		case <-ctx.Done():
			return nil
		case e := <-r.queue.transformerBatches:
			r.ProcessQueueOnce(ctx, e, defaultPushUpdate, DefaultPushActionCallback)
		}
	}
}

func (r *repository) applyTransformerBatches(transformerBatches []transformerBatch, allowFetchAndReset bool) ([]transformerBatch, error, *TransformerResult) {
	//exhaustruct:ignore
	var changes = &TransformerResult{}

	for i := 0; i < len(transformerBatches); {
		var transaction *sql.Tx
		var txErr error
		e := transformerBatches[i]
		if r.DB.ShouldUseEslTable() {
			transaction, txErr = r.DB.DB.BeginTx(e.ctx, nil)
			if txErr != nil {
				e.finish(txErr)
				transformerBatches = append(transformerBatches[:i], transformerBatches[i+1:]...)
				continue //Skip this batch
			}
		}
		subChanges, applyErr := r.ApplyTransformers(e.ctx, transaction, e.transformers...)
		if applyErr != nil {
			//Some transformer on this batch failed. Rollback the transaction
			if r.DB.ShouldUseEslTable() {
				rollBackError := transaction.Rollback()
				if rollBackError != nil {
					e.finish(rollBackError)
					transformerBatches = append(transformerBatches[:i], transformerBatches[i+1:]...)
					continue
				}
			}
			if errors.Is(applyErr.TransformerError, InvalidJson) && allowFetchAndReset {
				// Invalid state. fetch and reset and redo
				err := r.FetchAndReset(e.ctx)
				if err != nil {
					return transformerBatches, err, nil
				}
				return r.applyTransformerBatches(transformerBatches, false)
			} else {
				e.finish(applyErr)
				transformerBatches = append(transformerBatches[:i], transformerBatches[i+1:]...)
			}
		} else {
			if r.DB.ShouldUseEslTable() {
				err := transaction.Commit()
				if err != nil {
					e.finish(err)
					transformerBatches = append(transformerBatches[:i], transformerBatches[i+1:]...)
					continue
				}
			}
			changes.Combine(subChanges)
			i++
		}
	}
	return transformerBatches, nil, changes
}

var panicError = errors.New("Panic")

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

func (r *repository) drainQueue(ctx context.Context) []transformerBatch {
	if r.config.MaximumCommitsPerPush < 2 {
		return nil
	}

	limit := r.config.MaximumCommitsPerPush - 1
	transformerBatches := []transformerBatch{}
	defer r.queue.GaugeQueueSize(ctx)
	for uint(len(transformerBatches)) < limit {
		select {
		case f := <-r.queue.transformerBatches:
			// Check that the item is not already cancelled
			select {
			case <-f.ctx.Done():
				f.finish(f.ctx.Err())
			default:
				transformerBatches = append(transformerBatches, f)
			}
		default:
			return transformerBatches
		}
	}
	return transformerBatches
}

func (r *repository) GaugeQueueSize(ctx context.Context) {
	r.queue.GaugeQueueSize(ctx)
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

func (r *repository) ProcessQueueOnce(ctx context.Context, e transformerBatch, callback PushUpdateFunc, pushAction PushActionCallbackFunc) {
	logger := logger.FromContext(ctx)
	span, ctx := tracer.StartSpanFromContext(ctx, "ProcessQueueOnce")
	defer span.Finish()
	/**
	Note that this function has a bit different error handling.
	The error is not returned, but send to the transformer in `el.finish(err)`
	in order to inform the transformers request handler that this request failed.
	Therefore, in the function instead of
	if err != nil {
	  return err
	}
	we do:
	if err != nil {
	  return
	}
	*/
	var err error = panicError

	// Check that the first transformerBatch is not already canceled
	select {
	case <-e.ctx.Done():
		e.finish(e.ctx.Err())
		return
	default:
	}

	transformerBatches := []transformerBatch{e}
	defer func() {
		for _, el := range transformerBatches {
			el.finish(err)
		}
	}()

	// Try to fetch more items from the queue in order to push more things together
	transformerBatches = append(transformerBatches, r.drainQueue(ctx)...)

	var pushSuccess = true

	//exhaustruct:ignore
	RemoteCallbacks := git.RemoteCallbacks{
		CredentialsCallback:         r.credentials.CredentialsCallback(e.ctx),
		CertificateCheckCallback:    r.certificates.CertificateCheckCallback(e.ctx),
		PushUpdateReferenceCallback: callback(r.config.Branch, &pushSuccess),
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

	transformerBatches, err, changes := r.applyTransformerBatches(transformerBatches, true)

	if len(transformerBatches) == 0 {
		return
	}

	logger.Sugar().Infof("applyTransformerBatches: Attempting to push %d transformer batches to manifest repo.\n", len(transformerBatches))
	// Try pushing once
	err = r.Push(e.ctx, pushAction(pushOptions, r))
	if err != nil {
		gerr, ok := err.(*git.GitError)
		// If it doesn't work because the branch diverged, try reset and apply again.
		if ok && gerr.Code == git.ErrorCodeNonFastForward {
			err = r.FetchAndReset(e.ctx)
			if err != nil {
				return
			}
			transformerBatches, err, changes = r.applyTransformerBatches(transformerBatches, false)
			if err != nil || len(transformerBatches) == 0 {
				return
			}
			if pushErr := r.Push(e.ctx, pushAction(pushOptions, r)); pushErr != nil {
				err = pushErr
			}
		} else if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
			err = grpc.CanceledError(ctx, err)
		} else {
			logger.Error(fmt.Sprintf("error while pushing: %s", err))
			err = grpc.PublicError(ctx, fmt.Errorf("could not push to manifest repository '%s' on branch '%s' - this indicates that the ssh key does not have write access", r.config.URL, r.config.Branch))
		}
	} else {
		if !pushSuccess {
			err = fmt.Errorf("failed to push - this indicates that branch protection is enabled in '%s' on branch '%s'", r.config.URL, r.config.Branch)
		}
	}
	span, ctx = tracer.StartSpanFromContext(ctx, "PostPush")
	defer span.Finish()

	ddSpan, ctx := tracer.StartSpanFromContext(ctx, "SendMetrics")

	if r.config.DogstatsdEvents {
		var ddError error
		if r.DB.ShouldUseEslTable() {
			ddError = UpdateDatadogMetricsDB(ctx, r.State(), r, changes, time.Now())
		} else {
			ddError = UpdateDatadogMetrics(ctx, nil, r.State(), r, changes, time.Now())
		}
		if ddError != nil {
			logger.Warn(fmt.Sprintf("Could not send datadog metrics/events %v", ddError))
		}
	}
	ddSpan.Finish()

	if r.config.ArgoWebhookUrl != "" {
		r.sendWebhookToArgoCd(ctx, logger, changes)
	}

	r.notify.Notify()
}

func UpdateDatadogMetricsDB(ctx context.Context, state *State, r Repository, changes *TransformerResult, now time.Time) error {
	var transaction *sql.Tx
	var txErr error
	repo := r.(*repository)
	transaction, txErr = repo.DB.DB.BeginTx(ctx, nil)
	if txErr != nil {
		return txErr
	}
	defer func(tx *sql.Tx) {
		_ = tx.Rollback()
	}(transaction)

	ddError := UpdateDatadogMetrics(ctx, transaction, state, r, changes, now)
	if ddError != nil {
		return ddError
	}

	err := transaction.Commit()
	if err != nil {
		return err
	}
	return nil
}

func (r *repository) sendWebhookToArgoCd(ctx context.Context, logger *zap.Logger, changes *TransformerResult) {
	var modified = []string{}
	for i := range changes.ChangedApps {
		change := changes.ChangedApps[i]
		// we may need to add the root app in some circumstances - so far it doesn't seem necessary, so we just add the manifest.yaml:
		manifestFilename := fmt.Sprintf("environments/%s/applications/%s/manifests/manifests.yaml", change.Env, change.App)
		modified = append(modified, manifestFilename)
		logger.Info(fmt.Sprintf("ArgoWebhookUrl: adding modified: %s", manifestFilename))
	}
	var deleted = []string{}
	for i := range changes.DeletedRootApps {
		change := changes.DeletedRootApps[i]
		// we may need to add the root app in some circumstances - so far it doesn't seem necessary, so we just add the manifest.yaml:
		rootAppFilename := fmt.Sprintf("argocd/%s/%s.yaml", "v1alpha1", change.Env)
		deleted = append(deleted, rootAppFilename)
		logger.Info(fmt.Sprintf("ArgoWebhookUrl: adding modified: %s", rootAppFilename))
	}

	argoResult := ArgoWebhookData{
		htmlUrl:  r.config.WebURL, // if this does not match, argo will completely ignore the request and return 200
		revision: "refs/heads/" + r.config.Branch,
		change: changeInfo{
			payloadBefore: "",
			payloadAfter:  changes.Commits.Current.String(),
		},
		defaultBranch: r.config.Branch, // this is questionable, because we don't actually know the default branch, but it seems to work fine in practice
		Commits: []commit{
			{
				Added:    []string{},
				Modified: modified,
				Removed:  deleted,
			},
		},
	}
	if changes.Commits.Previous != nil {
		argoResult.change.payloadBefore = changes.Commits.Previous.String()
	}

	span, ctx := tracer.StartSpanFromContext(ctx, "Webhook-Retries")
	defer span.Finish()

	success := false
	var err error = nil
	for i := 1; i <= maxArgoRequests; i++ {
		err, shouldRetry := doWebhookPostRequest(ctx, argoResult, r.config, i)
		if err != nil && shouldRetry {
			logger.Warn(fmt.Sprintf("ProcessQueueOnce: error sending webhook on try %d: %v", i, err))
			if shouldRetry {
				// we're still in a request here, we can't wait too long:
				time.Sleep(time.Duration(100*i) * time.Millisecond)
			} else {
				break
			}
		} else {
			logger.Info(fmt.Sprintf("ProcessQueueOnce: argo webhook was send successfully on try %d!", i))
			success = true
			break
		}
	}
	span.SetTag("success", success)
	if !success {
		logger.Error(fmt.Sprintf("ProcessQueueOnce: error sending webhook after all %d tries: %v", maxArgoRequests, err))
	}
}

func contains(s []int, e int) bool {
	for _, a := range s {
		if a == e {
			return true
		}
	}
	return false
}

func doWebhookPostRequest(ctx context.Context, data ArgoWebhookData, repoConfig *RepositoryConfig, retryCounter int) (error, bool) {
	span, ctx := tracer.StartSpanFromContext(ctx, "Webhook")
	span.SetTag("changeAfter", data.change.payloadAfter)
	span.SetTag("changeBefore", data.change.payloadBefore)
	span.SetTag("try", retryCounter)
	defer span.Finish()
	url := repoConfig.ArgoWebhookUrl + "/api/webhook"
	l := logger.FromContext(ctx)
	l.Info(fmt.Sprintf("doWebhookPostRequest: URL: %s", url))

	//exhaustruct:ignore
	Repository := v1alpha1.Repository{
		HTMLURL:       data.htmlUrl,
		DefaultBranch: data.defaultBranch,
	}
	//exhaustruct:ignore
	var argoFormat = v1alpha1.PushPayload{
		Ref:        data.revision,
		Before:     data.change.payloadBefore,
		After:      data.change.payloadAfter,
		Repository: Repository,
		Commits:    toArgoCommits(data.Commits),
	}

	jsonBytes, err := json.MarshalIndent(argoFormat, " ", " ")
	if err != nil {
		return err, false
	}
	l.Info(fmt.Sprintf("doWebhookPostRequest argo format: %s", string(jsonBytes)))
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonBytes))
	if err != nil {
		return fmt.Errorf("Could not create new request: %s", err.Error()), false
	}
	req.Header.Set("Content-Type", "application/json")

	// now pretend that we are GitHub by adding this header, otherwise argo will ignore our request:
	req.Header.Set("X-GitHub-Event", "push")

	var webhookResolver WebhookResolver = DefaultWebhookResolver{}
	if repoConfig.WebhookResolver != nil {
		webhookResolver = repoConfig.WebhookResolver
	}
	resp, err := webhookResolver.Resolve(repoConfig.ArgoInsecure, req)
	if err != nil {
		return fmt.Errorf("doWebhookPostRequest: could not send request to '%s': %s", url, err.Error()), false
	}
	defer resp.Body.Close()

	//l.Warn(fmt.Sprintf("response Status: %d", resp.StatusCode))
	l.Info(fmt.Sprintf("response headers: %s", resp.Header))
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		// weird but we kinda do not care about the body:
		l.Warn(fmt.Sprintf("doWebhookPostRequest: could not read body: %s - continuing anyway", err.Error()))
	}
	validResponseCodes := []int{200}
	if resp.StatusCode >= 500 {
		return fmt.Errorf("doWebhookPostRequest: invalid status code from argo: %d", resp.StatusCode), true
	}

	if contains(validResponseCodes, resp.StatusCode) {
		l.Info(fmt.Sprintf("doWebhookPostRequest: response Body: %s", string(body)))
		return nil, false
	}
	// in any other case we should not do a retry (e.g. status 4xx):
	l.Warn(fmt.Sprintf("doWebhookPostRequest: response Body: %s", string(body)))
	return fmt.Errorf("doWebhookPostRequest: invalid status code from argo: %d", resp.StatusCode), false
}

func toArgoCommits(commits []commit) []v1alpha1.Commit {
	var result = []v1alpha1.Commit{}
	for i := range commits {
		c := commits[i]
		result = append(result, v1alpha1.Commit{
			// ArgoCd ignores most fields, so we can ignore them too.
			// Source: function "affectedRevisionInfo" in https://github.com/argoproj/argo-cd/blob/master/util/webhook/webhook.go#L141
			Sha:       "",
			ID:        "",
			NodeID:    "",
			TreeID:    "",
			Distinct:  false,
			Message:   "",
			Timestamp: "",
			URL:       "",
			Author: struct {
				Name     string `json:"name"`
				Email    string `json:"email"`
				Username string `json:"username"`
			}{
				Name:     "",
				Email:    "",
				Username: "",
			},
			Committer: struct {
				Name     string `json:"name"`
				Email    string `json:"email"`
				Username string `json:"username"`
			}{
				Name:     "",
				Email:    "",
				Username: "",
			},
			Added:    c.Added,
			Removed:  c.Removed,
			Modified: c.Modified,
		})
	}
	return result
}

type changeInfo struct {
	payloadBefore string
	payloadAfter  string
}
type commit struct {
	Added    []string
	Modified []string
	Removed  []string
}

type ArgoWebhookData struct {
	htmlUrl       string
	revision      string // aka "ref"
	change        changeInfo
	defaultBranch string
	Commits       []commit
}

func (r *repository) ApplyTransformersInternal(ctx context.Context, transaction *sql.Tx, transformers ...Transformer) ([]string, *State, []*TransformerResult, *TransformerBatchApplyError) {
	if state, err := r.StateAt(nil); err != nil {
		return nil, nil, nil, &TransformerBatchApplyError{TransformerError: fmt.Errorf("%s: %w", "failure in StateAt", err), Index: -1}
	} else {
		var changes []*TransformerResult = nil
		commitMsg := []string{}
		ctxWithTime := time2.WithTimeNow(ctx, time.Now())
		for i, t := range transformers {
			if r.DB != nil && transaction == nil {
				applyErr := TransformerBatchApplyError{
					TransformerError: errors.New("no transaction provided, but DB enabled"),
					Index:            i,
				}
				return nil, nil, nil, &applyErr
			}
			logger.FromContext(ctx).Info("writing esl event...")
			user, readUserErr := auth.ReadUserFromContext(ctx)

			if readUserErr != nil {
				return nil, nil, nil, &TransformerBatchApplyError{
					TransformerError: readUserErr,
					Index:            -1,
				}
			}
			if r.DB.ShouldUseOtherTables() {
				ev, err := r.DB.DBReadEslEventInternal(ctx, transaction, false)
				if err != nil {
					return nil, nil, nil, &TransformerBatchApplyError{
						TransformerError: err,
						Index:            i,
					}
				}
				var id uint
				if ev == nil {
					id = 1
				} else {
					id = uint(ev.EslId) + 1
				}
				t.SetEslID(id)
			}

			eventMetadata := db.ESLMetadata{
				AuthorName:  user.Name,
				AuthorEmail: user.Email,
			}
			err = r.DB.DBWriteEslEventInternal(ctx, t.GetDBEventType(), transaction, t, eventMetadata)
			if err != nil {
				return nil, nil, nil, &TransformerBatchApplyError{
					TransformerError: err,
					Index:            i,
				}
			}
			if msg, subChanges, err := RunTransformer(ctxWithTime, t, state, transaction); err != nil {
				applyErr := TransformerBatchApplyError{
					TransformerError: err,
					Index:            i,
				}
				return nil, nil, nil, &applyErr
			} else {
				commitMsg = append(commitMsg, msg)
				changes = append(changes, subChanges)
			}
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

func (r *repository) ApplyTransformers(ctx context.Context, transaction *sql.Tx, transformers ...Transformer) (*TransformerResult, *TransformerBatchApplyError) {
	commitMsg, state, changes, applyErr := r.ApplyTransformersInternal(ctx, transaction, transformers...)
	if applyErr != nil {
		return nil, applyErr
	}
	if err := r.afterTransform(ctx, *state, transaction); err != nil {
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

	user, readUserErr := auth.ReadUserFromContext(ctx)

	if readUserErr != nil {
		return nil, &TransformerBatchApplyError{
			TransformerError: readUserErr,
			Index:            -1,
		}
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

func (r *repository) Apply(ctx context.Context, transformers ...Transformer) error {
	defer func() {
		r.writesDone = r.writesDone + uint(len(transformers))
		r.maybeGc(ctx)
	}()
	eCh := r.applyDeferred(ctx, transformers...)
	select {
	case err := <-eCh:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (r *repository) applyDeferred(ctx context.Context, transformers ...Transformer) <-chan error {
	return r.queue.add(ctx, transformers)
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

func (r *repository) afterTransform(ctx context.Context, state State, transaction *sql.Tx) error {
	span, ctx := tracer.StartSpanFromContext(ctx, "afterTransform")
	defer span.Finish()

	configs, err := state.GetAllEnvironmentConfigs(ctx, transaction)
	if err != nil {
		return err
	}
	for env, config := range configs {
		if config.ArgoCd != nil {
			err := r.updateArgoCdApps(ctx, &state, env, config, transaction)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func (r *repository) updateArgoCdApps(ctx context.Context, state *State, env string, config config.EnvironmentConfig, transaction *sql.Tx) error {
	span, ctx := tracer.StartSpanFromContext(ctx, "updateArgoCdApps")
	defer span.Finish()
	fs := state.Filesystem
	if apps, err := state.GetEnvironmentApplications(ctx, transaction, env); err != nil {
		return err
	} else {
		spanCollectData, _ := tracer.StartSpanFromContext(ctx, "collectData")
		defer spanCollectData.Finish()
		appData := []argocd.AppData{}
		sort.Strings(apps)
		for _, appName := range apps {
			if err != nil {
				return err
			}
			team, err := state.GetApplicationTeamOwner(ctx, transaction, appName)
			if err != nil {
				return err
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
				TeamName: team,
			})
		}
		spanCollectData.Finish()

		if r.config.ArgoCdGenerateFiles {
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
						Commit:                 nil,
						Filesystem:             fs.NewEmptyTreeBuildFS(r.repository),
						BootstrapMode:          r.config.BootstrapMode,
						EnvironmentConfigsPath: r.config.EnvironmentConfigsPath,
						ReleaseVersionsLimit:   r.config.ReleaseVersionsLimit,
						DBHandler:              r.DB,
						CloudRunClient:         r.config.CloudRunClient,
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
		Filesystem:             fs.NewTreeBuildFS(r.repository, commit.TreeId()),
		Commit:                 commit,
		BootstrapMode:          r.config.BootstrapMode,
		EnvironmentConfigsPath: r.config.EnvironmentConfigsPath,
		ReleaseVersionsLimit:   r.config.ReleaseVersionsLimit,
		DBHandler:              r.DB,
		CloudRunClient:         r.config.CloudRunClient,
	}, nil
}

func (r *repository) Notify() *notify.Notify {
	return &r.notify
}

type ObjectCount struct {
	Count       uint64
	Size        uint64
	InPack      uint64
	Packs       uint64
	SizePack    uint64
	Garbage     uint64
	SizeGarbage uint64
}

func (r *repository) countObjects(ctx context.Context) (ObjectCount, error) {
	var stats ObjectCount
	/*
		The output of `git count-objects` looks like this:
			count: 0
			size: 0
			in-pack: 635
			packs: 1
			size-pack: 2845
			prune-packable: 0
			garbage: 0
			size-garbage: 0
	*/
	cmd := exec.CommandContext(ctx, "git", "count-objects", "--verbose")
	cmd.Dir = r.config.Path
	out, err := cmd.Output()
	if err != nil {
		return stats, err
	}
	scanner := bufio.NewScanner(bytes.NewReader(out))
	for scanner.Scan() {
		var (
			token string
			value uint64
		)
		if _, err := fmt.Sscan(scanner.Text(), &token, &value); err != nil {
			return stats, err
		}
		switch token {
		case "count:":
			stats.Count = value
		case "size:":
			stats.Size = value
		case "in-packs:":
			stats.InPack = value
		case "packs:":
			stats.Packs = value
		case "size-pack:":
			stats.SizePack = value
		case "garbage:":
			stats.Garbage = value
		case "size-garbage":
			stats.SizeGarbage = value
		}
	}
	return stats, nil
}

func (r *repository) maybeGc(ctx context.Context) {
	if r.config.StorageBackend == SqliteBackend || r.config.GcFrequency == 0 || r.writesDone < r.config.GcFrequency {
		return
	}
	log := logger.FromContext(ctx)
	r.writesDone = 0
	timeBefore := time.Now()
	statsBefore, _ := r.countObjects(ctx)
	cmd := exec.CommandContext(ctx, "git", "repack", "-a", "-d")
	cmd.Dir = r.config.Path
	err := cmd.Run()
	if err != nil {
		log.Fatal("git.repack", zap.Error(err))
		return
	}
	statsAfter, _ := r.countObjects(ctx)
	log.Info("git.repack", zap.Duration("duration", time.Since(timeBefore)), zap.Uint64("collected", statsBefore.Count-statsAfter.Count))
}

type State struct {
	Filesystem             billy.Filesystem
	Commit                 *git.Commit
	BootstrapMode          bool
	EnvironmentConfigsPath string
	ReleaseVersionsLimit   uint
	// DbHandler will be nil if the DB is disabled
	DBHandler      *db.DBHandler
	CloudRunClient *cloudrun.CloudRunClient
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
	if transaction == nil {
		return nil, fmt.Errorf("GetEnvironmentLocksFromDB: No transaction provided")
	}
	allActiveLockIds, err := s.DBHandler.DBSelectAllEnvironmentLocks(ctx, transaction, environment)
	if err != nil {
		return nil, err
	}
	var lockIds []string
	if allActiveLockIds != nil {
		lockIds = allActiveLockIds.EnvLocks
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

func (s *State) GetEnvironmentLocks(ctx context.Context, transaction *sql.Tx, environment string) (map[string]Lock, error) {
	if s.DBHandler.ShouldUseOtherTables() {
		return s.GetEnvironmentLocksFromDB(ctx, transaction, environment)
	}
	return s.GetEnvironmentLocksFromManifest(environment)
}

func (s *State) GetEnvironmentLocksFromManifest(environment string) (map[string]Lock, error) {
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

func (s *State) GetEnvironmentApplicationLocks(ctx context.Context, transaction *sql.Tx, environment, application string) (map[string]Lock, error) {
	if s.DBHandler.ShouldUseOtherTables() {
		return s.GetEnvironmentApplicationLocksFromDB(ctx, transaction, environment, application)
	}
	return s.GetEnvironmentApplicationLocksFromManifest(environment, application)
}

func (s *State) GetEnvironmentApplicationLocksFromDB(ctx context.Context, transaction *sql.Tx, environment, application string) (map[string]Lock, error) {
	if transaction == nil {
		return nil, fmt.Errorf("GetEnvironmentApplicationLocksFromDB: No transaction provided")
	}
	activeLockIds, err := s.DBHandler.DBSelectAllAppLocks(ctx, transaction, environment, application)
	if err != nil {
		return nil, err
	}
	var lockIds []string
	if activeLockIds != nil {
		lockIds = activeLockIds.AppLocks
	}
	locks, err := s.DBHandler.DBSelectAppLockSet(ctx, transaction, environment, application, lockIds)

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

func (s *State) GetEnvironmentApplicationLocksFromManifest(environment, application string) (map[string]Lock, error) {
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

func (s *State) GetEnvironmentTeamLocks(ctx context.Context, transaction *sql.Tx, environment, team string) (map[string]Lock, error) {
	if s.DBHandler.ShouldUseOtherTables() {
		return s.GetEnvironmentTeamLocksFromDB(ctx, transaction, environment, team)
	}
	return s.GetEnvironmentTeamLocksFromManifest(environment, team)
}

func (s *State) GetEnvironmentTeamLocksFromDB(ctx context.Context, transaction *sql.Tx, environment, team string) (map[string]Lock, error) {
	if transaction == nil {
		return nil, fmt.Errorf("GetEnvironmentTeamLocksFromDB: No transation provided")
	}
	activeLockIDs, err := s.DBHandler.DBSelectAllTeamLocks(ctx, transaction, environment, team)
	if err != nil {
		return nil, err
	}

	var lockIds []string
	if activeLockIDs != nil {
		lockIds = activeLockIDs.TeamLocks
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

func (s *State) GetEnvironmentTeamLocksFromManifest(environment, team string) (map[string]Lock, error) {
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
func (s *State) GetDeploymentMetaData(ctx context.Context, transaction *sql.Tx, environment, application string) (string, time.Time, error) {
	if s.DBHandler.ShouldUseOtherTables() {
		result, err := s.DBHandler.DBSelectDeployment(ctx, transaction, application, environment)
		if err != nil {
			return "", time.Time{}, err
		}
		if result != nil {
			return result.Metadata.DeployedByEmail, result.Created, nil
		}
		return "", time.Time{}, err
	}
	return s.GetDeploymentMetaDataFromRepo(environment, application)
}

func (s *State) GetDeploymentMetaDataFromRepo(environment, application string) (string, time.Time, error) {
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

func (s *State) GetQueuedVersionFromDB(ctx context.Context, transaction *sql.Tx, environment string, application string) (*uint64, error) {
	queuedDeployment, err := s.DBHandler.DBSelectLatestDeploymentAttempt(ctx, transaction, environment, application)

	if err != nil || queuedDeployment == nil {
		return nil, err
	}

	var v *uint64
	if queuedDeployment.Version != nil {
		parsedInt := uint64(*queuedDeployment.Version)
		v = &parsedInt
	} else {
		v = nil
	}
	return v, nil
}

func (s *State) GetQueuedVersion(ctx context.Context, transaction *sql.Tx, environment string, application string) (*uint64, error) {
	if s.DBHandler.ShouldUseOtherTables() {
		return s.GetQueuedVersionFromDB(ctx, transaction, environment, application)
	}
	return s.GetQueuedVersionFromManifest(environment, application)
}

func (s *State) GetQueuedVersionFromManifest(environment string, application string) (*uint64, error) {
	return s.readSymlink(environment, application, queueFileName)
}

func (s *State) DeleteQueuedVersionFromDB(ctx context.Context, transaction *sql.Tx, environment string, application string) error {
	return s.DBHandler.DBDeleteDeploymentAttempt(ctx, transaction, environment, application)
}

func (s *State) DeleteQueuedVersion(ctx context.Context, transaction *sql.Tx, environment string, application string) error {
	if s.DBHandler.ShouldUseOtherTables() {
		return s.DeleteQueuedVersionFromDB(ctx, transaction, environment, application)
	}
	queuedVersion := s.Filesystem.Join("environments", environment, "applications", application, queueFileName)
	return s.Filesystem.Remove(queuedVersion)
}

func (s *State) DeleteQueuedVersionIfExists(ctx context.Context, transaction *sql.Tx, environment string, application string) error {
	queuedVersion, err := s.GetQueuedVersion(ctx, transaction, environment, application)
	if err != nil {
		return err
	}
	if queuedVersion == nil {
		return nil // nothing to do
	}
	return s.DeleteQueuedVersion(ctx, transaction, environment, application)
}

func (s *State) GetEnvironmentApplicationVersion(ctx context.Context, transaction *sql.Tx, environment string, application string) (*uint64, error) {
	if s.DBHandler.ShouldUseOtherTables() && transaction != nil {
		depl, err := s.DBHandler.DBSelectDeployment(ctx, transaction, application, environment)
		if err != nil {
			return nil, err
		}
		if depl == nil || depl.Version == nil {
			return nil, nil
		}
		var v = uint64(*depl.Version)
		return &v, nil
	} else {
		return s.GetEnvironmentApplicationVersionFromManifest(environment, application)
	}
}

func (s *State) GetEnvironmentApplicationVersionFromManifest(environment string, application string) (*uint64, error) {
	return s.readSymlink(environment, application, "version")
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

func (s *State) GetTeamName(ctx context.Context, transaction *sql.Tx, application string) (string, error) {
	return s.GetApplicationTeamOwner(ctx, transaction, application)
}

func (s *State) GetTeamNameFromManifest(application string) (string, error) {
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

func (s *State) GetEnvironmentConfigsAndValidate(ctx context.Context, transaction *sql.Tx) (map[string]config.EnvironmentConfig, error) {
	logger := logger.FromContext(ctx)
	envConfigs, err := s.GetAllEnvironmentConfigs(ctx, transaction)
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

func (s *State) GetEnvironmentConfigsSorted(ctx context.Context, transaction *sql.Tx) (map[string]config.EnvironmentConfig, []string, error) {
	configs, err := s.GetAllEnvironmentConfigs(ctx, transaction)
	if err != nil {
		return nil, nil, err
	}
	// sorting the environments to get a deterministic order of events:
	var envNames []string = nil
	for envName := range configs {
		envNames = append(envNames, envName)
	}
	sort.Strings(envNames)
	return configs, envNames, nil
}

func (s *State) GetEnvironmentConfigsSortedFromManifest() (map[string]config.EnvironmentConfig, []string, error) {
	configs, err := s.GetAllEnvironmentConfigsFromManifest()
	if err != nil {
		return nil, nil, err
	}
	// sorting the environments to get a deterministic order of events:
	var envNames []string = nil
	for envName := range configs {
		envNames = append(envNames, envName)
	}
	sort.Strings(envNames)
	return configs, envNames, nil
}

func (s *State) GetAllEnvironmentConfigs(ctx context.Context, transaction *sql.Tx) (map[string]config.EnvironmentConfig, error) {
	if s.DBHandler.ShouldUseOtherTables() {
		return s.GetAllEnvironmentConfigsFromDB(ctx, transaction)
	}
	return s.GetAllEnvironmentConfigsFromManifest()
}

func (s *State) GetAllEnvironmentConfigsFromManifest() (map[string]config.EnvironmentConfig, error) {
	if s.BootstrapMode {
		result := map[string]config.EnvironmentConfig{}
		buf, err := os.ReadFile(s.EnvironmentConfigsPath)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				return result, nil
			}
			return nil, err
		}
		err = json.Unmarshal(buf, &result)
		if err != nil {
			return nil, err
		}
		return result, nil
	} else {
		envs, err := s.Filesystem.ReadDir("environments")
		if err != nil {
			return nil, err
		}
		result := map[string]config.EnvironmentConfig{}
		for _, env := range envs {
			c, err := s.GetEnvironmentConfigFromManifest(env.Name())
			if err != nil {
				return nil, err

			}
			result[env.Name()] = *c
		}
		return result, nil
	}
}

func (s *State) GetAllEnvironmentConfigsFromDB(ctx context.Context, transaction *sql.Tx) (map[string]config.EnvironmentConfig, error) {
	if s.BootstrapMode {
		// this should never ever happen
		return nil, fmt.Errorf("bootstrap mode cannot be enabled when writing to the database")
	} else {
		dbAllEnvs, err := s.DBHandler.DBSelectAllEnvironments(ctx, transaction)
		if err != nil {
			return nil, fmt.Errorf("unable to retrieve all environments, error: %w", err)
		}
		if dbAllEnvs == nil {
			return nil, nil
		}
		ret := make(map[string]config.EnvironmentConfig)
		for _, envName := range dbAllEnvs.Environments {
			dbEnv, err := s.DBHandler.DBSelectEnvironment(ctx, transaction, envName)
			if err != nil {
				return nil, fmt.Errorf("unable to retrieve manifest for environment %s from the database, error: %w", envName, err)
			}
			if dbEnv == nil {
				return nil, fmt.Errorf("the all_environments and environments tables are inconsistent in the database, environment %s was listed in the all_environments tables, but is not found in the environments table", envName)
			}
			ret[envName] = dbEnv.Config
		}
		if err != nil {
			return nil, err
		}
		return ret, nil
	}
}

// for use with custom migrations, otherwise use the two functions above
func (s *State) GetAllEnvironments(ctx context.Context) (map[string]config.EnvironmentConfig, error) {
	result := map[string]config.EnvironmentConfig{}

	fs := s.Filesystem

	envDir, err := fs.ReadDir("environments")
	if err != nil {
		return nil, fmt.Errorf("error while reading the environments directory, error: %w", err)
	}

	for _, envName := range envDir {
		configFilePath := fs.Join("environments", envName.Name(), "config.json")
		configBytes, err := readFile(fs, configFilePath)
		if err != nil {
			return nil, fmt.Errorf("could not read file at %s, error: %w", configFilePath, err)
		}
		//exhaustruct:ignore
		config := config.EnvironmentConfig{}
		err = json.Unmarshal(configBytes, &config)
		if err != nil {
			return nil, fmt.Errorf("error while unmarshaling the database JSON, error: %w", err)
		}
		result[envName.Name()] = config
	}

	return result, nil
}

func (s *State) GetEnvironmentConfig(ctx context.Context, transaction *sql.Tx, environmentName string) (*config.EnvironmentConfig, error) {
	if s.DBHandler.ShouldUseOtherTables() {
		return s.GetEnvironmentConfigFromDB(ctx, transaction, environmentName)
	}
	return s.GetEnvironmentConfigFromManifest(environmentName)
}

func (s *State) GetEnvironmentConfigFromManifest(environmentName string) (*config.EnvironmentConfig, error) {
	fileName := s.Filesystem.Join("environments", environmentName, "config.json")
	var config config.EnvironmentConfig
	if err := decodeJsonFile(s.Filesystem, fileName, &config); err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return nil, fmt.Errorf("%s : %w", fileName, InvalidJson)
		}
	}
	return &config, nil
}

func (s *State) GetEnvironmentConfigFromDB(ctx context.Context, transaction *sql.Tx, environmentName string) (*config.EnvironmentConfig, error) {
	dbEnv, err := s.DBHandler.DBSelectEnvironment(ctx, transaction, environmentName)
	if err != nil {
		return nil, fmt.Errorf("error while selecting entry for environment %s from the database, error: %w", environmentName, err)
	}
	if dbEnv == nil {
		return nil, nil
	}

	return &dbEnv.Config, nil
}

func (s *State) GetEnvironmentConfigsForGroup(ctx context.Context, transaction *sql.Tx, envGroup string) ([]string, error) {
	allEnvConfigs, err := s.GetAllEnvironmentConfigs(ctx, transaction)
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
	if s.DBHandler.ShouldUseOtherTables() && transaction != nil {
		return s.GetEnvironmentApplicationsFromDB(ctx, transaction, environment)
	}
	return s.GetEnvironmentApplicationsFromManifest(environment)
}

func (s *State) GetEnvironmentApplicationsFromManifest(environment string) ([]string, error) {
	appDir := s.Filesystem.Join("environments", environment, "applications")
	return names(s.Filesystem, appDir)
}

func (s *State) GetEnvironmentApplicationsFromDB(ctx context.Context, transaction *sql.Tx, environment string) ([]string, error) {
	applications, err := s.DBHandler.DBSelectAllApplications(ctx, transaction)
	if err != nil {
		return nil, err
	}
	if applications == nil {
		return make([]string, 0), nil
	}
	return applications.Apps, nil
}

// GetApplicationsFromFile returns all apps that exist in any env
func (s *State) GetApplicationsFromFile() ([]string, error) {
	return names(s.Filesystem, "applications")
}

// GetApplicationsFromFile returns all apps that exist in any env
func (s *State) GetApplications(ctx context.Context, transaction *sql.Tx) ([]string, error) {
	if s.DBHandler.ShouldUseOtherTables() {
		applications, err := s.DBHandler.DBSelectAllApplications(ctx, transaction)
		if err != nil {
			return nil, err
		}
		if applications == nil {
			return make([]string, 0), nil
		}
		return applications.Apps, nil
	} else {
		return s.GetApplicationsFromFile()
	}
}

// GetCurrentlyDeployed returns all apps that have current deployments on any env from the filesystem
func (s *State) GetCurrentlyDeployed(ctx context.Context, transaction *sql.Tx) (db.AllDeployments, error) {
	ddSpan, ctx := tracer.StartSpanFromContext(ctx, "GetCurrentlyDeployed")
	defer ddSpan.Finish()
	var result = db.AllDeployments{}
	_, envNames, err := s.GetEnvironmentConfigsSortedFromManifest() // this is intentional, when doing custom migrations (which is where this function is called), we want to read from the manifest repo explicitly
	if err != nil {
		return nil, err
	}
	for envNameIndex := range envNames {
		envName := envNames[envNameIndex]

		if apps, err := s.GetEnvironmentApplications(ctx, transaction, envName); err != nil {
			return nil, err
		} else {
			for _, appName := range apps {
				var version *uint64
				version, err = s.GetEnvironmentApplicationVersionFromManifest(envName, appName)
				if err != nil {
					return nil, fmt.Errorf("could not get version of app %s in env %s", appName, envName)
				}
				var versionIntPtr *int64
				if version != nil {
					var versionInt = int64(*version)
					versionIntPtr = &versionInt
				} else {
					versionIntPtr = nil
				}
				result = append(result, db.Deployment{
					EslVersion: 0,
					Created:    time.Time{},
					App:        appName,
					Env:        envName,
					Version:    versionIntPtr,
					Metadata: db.DeploymentMetadata{
						DeployedByName:  "",
						DeployedByEmail: "",
					},
				})
			}
		}
	}
	return result, nil
}

// GetCurrentEnvironmentLocks gets all locks on any environment in manifest
func (s *State) GetCurrentEnvironmentLocks(ctx context.Context) (db.AllEnvLocks, error) {
	ddSpan, _ := tracer.StartSpanFromContext(ctx, "GetCurrentEnvironmentLocks")
	defer ddSpan.Finish()
	result := make(db.AllEnvLocks)
	_, envNames, err := s.GetEnvironmentConfigsSortedFromManifest() // this is intentional, when doing custom migrations (which is where this function is called), we want to read from the manifest repo explicitly
	if err != nil {
		return nil, err
	}
	for envNameIndex := range envNames {
		envName := envNames[envNameIndex]
		var currentEnv []db.EnvironmentLock

		ls, err := s.GetEnvironmentLocksFromManifest(envName)
		if err != nil {
			return nil, err
		}
		for lockId, lock := range ls {
			currentEnv = append(currentEnv, db.EnvironmentLock{
				EslVersion: 0,
				Env:        envName,
				LockID:     lockId,
				Created:    lock.CreatedAt,
				Metadata: db.LockMetadata{
					CreatedByName:  lock.CreatedBy.Name,
					CreatedByEmail: lock.CreatedBy.Email,
					Message:        lock.Message,
				},
				Deleted: false,
			})
		}
		result[envName] = currentEnv
	}
	return result, nil
}

func (s *State) GetCurrentApplicationLocks(ctx context.Context) (db.AllAppLocks, error) {
	ddSpan, _ := tracer.StartSpanFromContext(ctx, "GetCurrentApplicationLocks")
	defer ddSpan.Finish()
	result := make(db.AllAppLocks)
	_, envNames, err := s.GetEnvironmentConfigsSortedFromManifest() // this is intentional, when doing custom migrations (which is where this function is called), we want to read from the manifest repo explicitly

	if err != nil {
		return nil, err
	}
	for envNameIndex := range envNames {

		envName := envNames[envNameIndex]

		appNames, err := s.GetEnvironmentApplicationsFromManifest(envName)
		if err != nil {
			return nil, err
		}

		result[envName] = make(map[string][]db.ApplicationLock)
		for _, currentApp := range appNames {
			var currentAppLocks []db.ApplicationLock
			ls, err := s.GetEnvironmentApplicationLocksFromManifest(envName, currentApp)
			if err != nil {
				return nil, err
			}
			for lockId, lock := range ls {
				currentAppLocks = append(currentAppLocks, db.ApplicationLock{
					EslVersion: 0,
					Env:        envName,
					LockID:     lockId,
					Created:    lock.CreatedAt,
					Metadata: db.LockMetadata{
						CreatedByName:  lock.CreatedBy.Name,
						CreatedByEmail: lock.CreatedBy.Email,
						Message:        lock.Message,
					},
					App:     currentApp,
					Deleted: false,
				})
			}
			result[envName][currentApp] = currentAppLocks
		}
	}
	return result, nil
}

func (s *State) GetAllQueuedAppVersions(ctx context.Context) (db.AllQueuedVersions, error) {
	ddSpan, _ := tracer.StartSpanFromContext(ctx, "GetAllQueuedAppVersions")
	defer ddSpan.Finish()
	result := make(db.AllQueuedVersions)
	_, envNames, err := s.GetEnvironmentConfigsSortedFromManifest()

	if err != nil {
		return nil, err
	}
	for envNameIndex := range envNames {

		envName := envNames[envNameIndex]

		appNames, err := s.GetEnvironmentApplicationsFromManifest(envName)
		if err != nil {
			return nil, err
		}

		result[envName] = make(map[string]*int64)
		for _, currentApp := range appNames {
			var version *uint64
			version, err := s.GetQueuedVersionFromManifest(envName, currentApp)
			if err != nil {
				return nil, err
			}

			var versionIntPtr *int64
			if version != nil {
				var versionInt = int64(*version)
				versionIntPtr = &versionInt
			} else {
				versionIntPtr = nil
			}
			result[envName][currentApp] = versionIntPtr

		}
	}
	return result, nil
}

func (s *State) GetAllCommitEvents(ctx context.Context) (db.AllCommitEvents, error) {
	ddSpan, _ := tracer.StartSpanFromContext(ctx, "GetAllCommitEvents")
	defer ddSpan.Finish()
	fs := s.Filesystem
	allCommitsPath := "commits"
	commitPrefixes, err := fs.ReadDir(allCommitsPath)
	if err != nil {
		return nil, fmt.Errorf("could not read commits dir: %v\n", err)
	}
	result := make(db.AllCommitEvents)
	for _, currentPrefix := range commitPrefixes {
		currentpath := fs.Join(allCommitsPath, currentPrefix.Name())
		commitSuffixes, err := fs.ReadDir(currentpath)
		if err != nil {
			return nil, fmt.Errorf("could not read commit directory '%s': %v", currentpath, err)
		}
		for _, currentSuffix := range commitSuffixes {
			var currentEvents []event.DBEventGo
			currentpath := fs.Join(fs.Join(currentpath, currentSuffix.Name(), "events"))
			potentialEventDirs, err := fs.ReadDir(currentpath)
			if err != nil {
				return nil, fmt.Errorf("could not read events directory '%s': %v", currentpath, err)
			}
			for i := range potentialEventDirs {
				oneEventDir := potentialEventDirs[i]
				if oneEventDir.IsDir() {
					fileName := oneEventDir.Name()

					eType, err := readFile(fs, fs.Join(fs.Join(currentpath, fileName), "eventType"))

					if err != nil {
						return nil, fmt.Errorf("could not read event type '%s': %v", fs.Join(currentpath, fileName), err)
					}

					fsEvent, err := event.Read(fs, fs.Join(currentpath, fileName))
					if err != nil {
						return nil, fmt.Errorf("could not read events %v", err)
					}
					currentEvents = append(currentEvents, event.DBEventGo{
						EventData: fsEvent,
						EventMetadata: event.Metadata{
							Uuid:      fileName,
							EventType: string(eType),
						},
					})
				}
			}
			result[strings.Join([]string{currentPrefix.Name(), currentSuffix.Name()}, "")] = currentEvents
		}
	}
	return result, nil
}

func (s *State) GetAppsAndTeams() (map[string]string, error) {
	result, err := s.GetApplicationsFromFile()
	if err != nil {
		return nil, fmt.Errorf("could not get apps from file: %v", err)
	}
	var teamByAppName = map[string]string{} // key: app, value: team
	for i := range result {
		app := result[i]

		team, err := s.GetTeamNameFromManifest(app)
		if err != nil {
			// some apps do not have teams, that's not an error
			teamByAppName[app] = ""
		} else {
			teamByAppName[app] = team
		}
	}
	return teamByAppName, nil
}

func (s *State) GetAllReleases(ctx context.Context, app string) (db.AllReleases, error) {
	releases, err := s.GetAllApplicationReleasesFromManifest(app)
	if err != nil {
		return nil, fmt.Errorf("cannot get releases of app %s: %v", app, err)
	}
	var result = db.AllReleases{}
	for i := range releases {
		releaseVersion := releases[i]
		repoRelease, err := s.GetApplicationReleaseFromManifest(app, releaseVersion)
		if err != nil {
			return nil, fmt.Errorf("cannot get app release of app %s and release %v: %v", app, releaseVersion, err)
		}
		manifests, err := s.GetApplicationReleaseManifestsFromManifest(app, releaseVersion)
		if err != nil {
			return nil, fmt.Errorf("cannot get manifest for app %s and release %v: %v", app, releaseVersion, err)
		}
		var manifestsMap = map[string]string{}
		for index := range manifests {
			manifest := manifests[index]
			manifestsMap[manifest.Environment] = manifest.Content
		}
		result[releaseVersion] = db.ReleaseWithManifest{
			Version:         releaseVersion,
			UndeployVersion: repoRelease.UndeployVersion,
			SourceAuthor:    repoRelease.SourceAuthor,
			SourceCommitId:  repoRelease.SourceCommitId,
			SourceMessage:   repoRelease.SourceMessage,
			CreatedAt:       repoRelease.CreatedAt,
			DisplayVersion:  repoRelease.DisplayVersion,
			Manifests:       manifestsMap,
		}
	}
	return result, nil
}

func (s *State) GetAllApplicationReleases(ctx context.Context, transaction *sql.Tx, application string) ([]uint64, error) {
	if s.DBHandler.ShouldUseOtherTables() {
		app, err := s.DBHandler.DBSelectAllReleasesOfApp(ctx, transaction, application)
		if err != nil {
			return nil, fmt.Errorf("could not get all releases of app %s: %v", application, err)
		}
		if app == nil {
			return nil, fmt.Errorf("could not get all releases of app %s (nil)", application)
		}
		res := conversion.ToUint64Slice(app.Metadata.Releases)
		return res, nil
	} else {
		return s.GetAllApplicationReleasesFromManifest(application)
	}
}

func (s *State) GetAllApplicationReleasesFromManifest(application string) ([]uint64, error) {
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

func (s *State) GetCurrentTeamLocks(ctx context.Context) (db.AllTeamLocks, error) {
	ddSpan, _ := tracer.StartSpanFromContext(ctx, "GetCurrentTeamLocks")
	defer ddSpan.Finish()
	result := make(db.AllTeamLocks)
	_, envNames, err := s.GetEnvironmentConfigsSortedFromManifest() // this is intentional, when doing custom migrations (which is where this function is called), we want to read from the manifest repo explicitly

	if err != nil {
		return nil, err
	}

	for envNameIndex := range envNames {
		processedTeams := map[string]bool{} //TeamName -> boolean (processed or not)
		envName := envNames[envNameIndex]

		appNames, err := s.GetEnvironmentApplicationsFromManifest(envName)
		if err != nil {
			return nil, err
		}

		result[envName] = make(map[string][]db.TeamLock)
		for _, currentApp := range appNames {
			var currentTeamLocks []db.TeamLock

			teamName, err := s.GetTeamNameFromManifest(currentApp)
			if err != nil {
				if errors.Is(err, os.ErrNotExist) {
					continue //If app has no team, we skip it
				}
				return nil, err
			}
			_, exists := processedTeams[teamName]
			if !exists {
				processedTeams[teamName] = true
			} else {
				continue
			}

			ls, err := s.GetEnvironmentTeamLocksFromManifest(envName, teamName)
			if err != nil {
				return nil, err
			}
			for lockId, lock := range ls {
				currentTeamLocks = append(currentTeamLocks, db.TeamLock{
					EslVersion: 0,
					Env:        envName,
					LockID:     lockId,
					Created:    lock.CreatedAt,
					Metadata: db.LockMetadata{
						CreatedByName:  lock.CreatedBy.Name,
						CreatedByEmail: lock.CreatedBy.Email,
						Message:        lock.Message,
					},
					Team:    teamName,
					Deleted: false,
				})
			}
			result[envName][teamName] = currentTeamLocks
		}
	}
	return result, nil
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

func (s *State) GetApplicationRelease(ctx context.Context, transaction *sql.Tx, application string, version uint64) (*Release, error) {
	if s.DBHandler.ShouldUseOtherTables() {
		env, err := s.DBHandler.DBSelectReleaseByVersion(ctx, transaction, application, version)
		if err != nil {
			return nil, fmt.Errorf("could not get release of app %s: %v", application, err)
		}
		if env == nil {
			return nil, nil
		}
		return &Release{
			Version:         env.ReleaseNumber,
			UndeployVersion: false,
			SourceAuthor:    env.Metadata.SourceAuthor,
			SourceCommitId:  env.Metadata.SourceCommitId,
			SourceMessage:   env.Metadata.SourceMessage,
			CreatedAt:       env.Created,
			DisplayVersion:  env.Metadata.DisplayVersion,
		}, nil
	} else {
		return s.GetApplicationReleaseFromManifest(application, version)
	}
}

func (s *State) GetApplicationReleaseFromManifest(application string, version uint64) (*Release, error) {
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

func (s *State) GetApplicationReleaseManifests(ctx context.Context, transaction *sql.Tx, application string, version uint64) (map[string]*api.Manifest, error) {
	manifests := map[string]*api.Manifest{}
	if s.DBHandler.ShouldUseOtherTables() {
		release, err := s.DBHandler.DBSelectReleaseByVersion(ctx, transaction, application, version)
		if err != nil {
			return nil, fmt.Errorf("could not get release for app %s with version %v: %w", application, version, err)
		}
		for index, mani := range release.Manifests.Manifests {
			manifests[index] = &api.Manifest{
				Environment: index,
				Content:     mani,
			}
		}
		return manifests, nil
	} else {
		return s.GetApplicationReleaseManifestsFromManifest(application, version)
	}
}

func (s *State) GetApplicationReleaseManifestsFromManifest(application string, version uint64) (map[string]*api.Manifest, error) {
	manifests := map[string]*api.Manifest{}
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

		manifests[entry.Name()] = &api.Manifest{
			Environment: entry.Name(),
			Content:     string(content),
		}
	}
	return manifests, nil
}

func (s *State) GetApplicationTeamOwner(ctx context.Context, transaction *sql.Tx, application string) (string, error) {
	if s.DBHandler.ShouldUseOtherTables() {
		app, err := s.DBHandler.DBSelectApp(ctx, transaction, application)
		if err != nil {
			return "", fmt.Errorf("could not get team of app %s: %v", application, err)
		}
		if app == nil {
			return "", fmt.Errorf("could not get team of app %s - could not find app", application)
		}
		return app.Metadata.Team, nil
	} else {
		return s.GetApplicationTeamOwnerFromManifest(application)
	}
}
func (s *State) GetApplicationTeamOwnerFromManifest(application string) (string, error) {
	appDir := applicationDirectory(s.Filesystem, application)
	appTeam := s.Filesystem.Join(appDir, "team")

	if team, err := readFile(s.Filesystem, appTeam); err != nil {
		if os.IsNotExist(err) {
			return "", nil
		} else {
			return "", fmt.Errorf("error while reading team owner file for application %v found: %w", application, err)
		}
	} else {
		return string(team), nil
	}
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
	queuedVersion, err := s.GetQueuedVersion(ctx, transaction, environment, application)
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
			err = s.DeleteQueuedVersion(ctx, transaction, environment, application)
			return fmt.Sprintf("deleted queued version %d because it was already deployed. app=%q env=%q", *queuedVersion, application, environment), err
		}
	}
	return queueDeploymentMessage, nil
}
