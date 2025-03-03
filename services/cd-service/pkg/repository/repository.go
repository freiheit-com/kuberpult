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
	"crypto/tls"
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"sort"
	"sync"
	"time"

	"github.com/freiheit-com/kuberpult/pkg/conversion"
	time2 "github.com/freiheit-com/kuberpult/pkg/time"

	"github.com/freiheit-com/kuberpult/pkg/db"
	"github.com/freiheit-com/kuberpult/pkg/mapper"

	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/DataDog/datadog-go/v5/statsd"
	backoff "github.com/cenkalti/backoff/v4"
	api "github.com/freiheit-com/kuberpult/pkg/api/v1"
	"github.com/freiheit-com/kuberpult/pkg/auth"
	"github.com/freiheit-com/kuberpult/pkg/config"
	"github.com/freiheit-com/kuberpult/pkg/setup"
	"github.com/freiheit-com/kuberpult/services/cd-service/pkg/cloudrun"
	"github.com/freiheit-com/kuberpult/services/cd-service/pkg/notify"
	"gopkg.in/DataDog/dd-trace-go.v1/ddtrace/tracer"

	"github.com/freiheit-com/kuberpult/pkg/logger"
)

type contextKey string

const DdMetricsKey contextKey = "ddMetrics"

// A Repository provides a multiple reader / single writer access to a git repository.
type Repository interface {
	Apply(ctx context.Context, transformers ...Transformer) error
	ApplyTransformersInternal(ctx context.Context, transaction *sql.Tx, transformers ...Transformer) ([]string, *State, []*TransformerResult, *TransformerBatchApplyError)
	State() *State
	StateAt() (*State, error)
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
	if target == nil || tgt == nil {
		return false
	}
	// now both target and err are guaranteed to be non-nil
	if err.Index != tgt.Index {
		return false
	}
	return errors.Is(err.TransformerError, tgt.TransformerError)
}

func (err *TransformerBatchApplyError) Unwrap() error {
	if err == nil {
		return nil
	}
	// Return the inner error.
	return err.TransformerError
}

func UnwrapUntilTransformerBatchApplyError(err error) *TransformerBatchApplyError {
	for {
		var applyErr *TransformerBatchApplyError
		if errors.As(err, &applyErr) {
			return applyErr
		}
		err2 := errors.Unwrap(err)
		if err2 == nil {
			// cannot unwrap any further
			return nil
		}
	}
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

type repository struct {
	// Mutex gurading the writer
	writeLock sync.Mutex
	queue     queue
	config    *RepositoryConfig

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
	CommitterEmail string
	CommitterName  string
	// default branch is master
	Branch string
	// network timeout
	NetworkTimeout time.Duration
	// number of app versions to keep a history of
	ReleaseVersionsLimit uint
	StorageBackend       StorageBackend
	// the url to the git repo, like the browser requires it (https protocol)
	WebURL                string
	DogstatsdEvents       bool
	WriteCommitData       bool
	WebhookResolver       WebhookResolver
	MaximumCommitsPerPush uint
	MaximumQueueSize      uint
	MaxNumThreads         uint
	// Extend maximum AppName length
	AllowLongAppNames bool

	ArgoCdGenerateFiles bool
	MinorRegexes        []*regexp.Regexp

	DBHandler      *db.DBHandler
	CloudRunClient *cloudrun.CloudRunClient

	DisableQueue bool
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

	var err error

	result := &repository{
		headLock:        sync.Mutex{},
		notify:          notify.Notify{},
		writeLock:       sync.Mutex{},
		config:          &cfg,
		queue:           makeQueueN(cfg.MaximumQueueSize),
		backOffProvider: defaultBackOffProvider,
		DB:              cfg.DBHandler,
	}
	result.headLock.Lock()

	defer result.headLock.Unlock()
	// check that we can build the current state
	state, err := result.StateAt()
	if err != nil {
		return nil, nil, err
	}

	// Check configuration for errors and abort early if any:
	_, err = db.WithTransactionT(state.DBHandler, ctx, db.DefaultNumRetries, true, func(ctx context.Context, transaction *sql.Tx) (*map[string]config.EnvironmentConfig, error) {
		ret, err := state.GetEnvironmentConfigsAndValidate(ctx, transaction)
		return &ret, err
	})

	if err != nil {
		return nil, nil, err
	}

	return result, result.ProcessQueue, nil
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
			r.ProcessQueueOnce(ctx, e)
		}
	}
}

func (r *repository) applyTransformerBatches(transformerBatches []transformerBatch, allowFetchAndReset bool) ([]transformerBatch, error, *TransformerResult) {
	//exhaustruct:ignore
	var changes = &TransformerResult{}

	for i := 0; i < len(transformerBatches); {
		e := transformerBatches[i]

		subChanges, txErr := db.WithTransactionT(r.DB, e.ctx, 2, false, func(ctx context.Context, transaction *sql.Tx) (*TransformerResult, error) {
			subChanges, applyErr := r.ApplyTransformers(ctx, transaction, e.transformers...)
			if applyErr != nil {
				return nil, applyErr
			}
			return subChanges, nil
		})

		r.notify.NotifyGitSyncStatus()

		if txErr != nil {
			logger.FromContext(e.ctx).Sugar().Warnf("txError in applyTransformerBatches: %w", txErr)
			e.finish(txErr)
			transformerBatches = append(transformerBatches[:i], transformerBatches[i+1:]...)
			continue //Skip this batch
		}
		changes.Combine(subChanges)
		i++
	}
	return transformerBatches, nil, changes
}

var panicError = errors.New("Panic")

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

func (r *repository) ProcessQueueOnce(ctx context.Context, e transformerBatch) {
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

	transformerBatches, err, changes := r.applyTransformerBatches(transformerBatches, true)
	if len(transformerBatches) == 0 {
		return
	}

	r.notify.Notify()
	r.notifyChangedApps(changes)
}

func (r *repository) ApplyTransformersInternal(ctx context.Context, transaction *sql.Tx, transformers ...Transformer) ([]string, *State, []*TransformerResult, *TransformerBatchApplyError) {
	span, ctx := tracer.StartSpanFromContext(ctx, "ApplyTransformersInternal")
	defer span.Finish()

	if state, err := r.StateAt(); err != nil {
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

			eventMetadata := db.ESLMetadata{
				AuthorName:  user.Name,
				AuthorEmail: user.Email,
			}
			transfomerId := db.EslVersion(0)
			err = r.DB.DBWriteEslEventInternal(ctx, t.GetDBEventType(), transaction, t, eventMetadata)
			if err != nil {
				return nil, nil, nil, &TransformerBatchApplyError{
					TransformerError: err,
					Index:            i,
				}
			}
			// read the last written event, so we can get the primary key (eslVersion):
			internal, err := r.DB.DBReadEslEventInternal(ctx, transaction, false)
			if err != nil {
				return nil, nil, nil, &TransformerBatchApplyError{
					TransformerError: err,
					Index:            i,
				}
			}
			if internal == nil {
				return nil, nil, nil, &TransformerBatchApplyError{
					TransformerError: fmt.Errorf("could not find esl event that was just inserted with event type %v", t.GetDBEventType()),
					Index:            i,
				}
			}
			t.SetEslVersion(db.TransformerID(internal.EslVersion))

			if r.DB != nil && r.DB.WriteEslOnly {
				// if we were previously running with `db.writeEslTableOnly=true`, but now are running with
				// `db.writeEslTableOnly=false` (which is the recommended way to enable the database),
				// then we would have many events in the event_sourcing_light table that have not been processed.
				// So, we write the cutoff if we are only writing to the esl table. Then, when the database is fully
				// enabled, the cutoff is found and determined to be the latest transformer. When this happens,
				// the export service takes over the duties of writing the cutoff

				err = db.DBWriteCutoff(r.DB, ctx, transaction, internal.EslVersion)
				if err != nil {
					applyErr := TransformerBatchApplyError{
						TransformerError: err,
						Index:            i,
					}
					return nil, nil, nil, &applyErr
				}
			}
			transfomerId = internal.EslVersion

			if msg, subChanges, err := RunTransformer(ctxWithTime, t, state, transaction); err != nil {
				applyErr := TransformerBatchApplyError{
					TransformerError: err,
					Index:            i,
				}
				return nil, nil, nil, &applyErr
			} else {
				commitMsg = append(commitMsg, msg)
				changes = append(changes, subChanges)
				//Sync Status Update
				envApps := make([]db.EnvApp, 0)
				for _, currentResult := range subChanges.ChangedApps {
					if currentResult.App == "" || currentResult.Env == "" {
						logger.FromContext(ctx).Sugar().Warnf("Empty changed app or environment: App = '%s', Env = '%s'", currentResult.App, currentResult.Env)
						continue
					}
					envApps = append(envApps, db.EnvApp{
						AppName: currentResult.App,
						EnvName: currentResult.Env,
					})
				}
				err := state.DBHandler.DBWriteNewSyncEventBulk(ctx, transaction, db.TransformerID(transfomerId), envApps, db.UNSYNCED)
				if err != nil {
					return nil, nil, nil, &TransformerBatchApplyError{
						TransformerError: fmt.Errorf("failed writing new sync events for transformer '%d': %w", int(transfomerId), err),
						Index:            -1,
					}
				}
				logger.FromContext(ctx).Sugar().Infof("Transformer modified %d app/envs", len(envApps))
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
	span, ctx := tracer.StartSpanFromContext(ctx, "ApplyTransformers")
	defer span.Finish()

	_, _, changes, applyErr := r.ApplyTransformersInternal(ctx, transaction, transformers...)
	if applyErr != nil {
		return nil, applyErr
	}
	result := CombineArray(changes)
	return result, nil
}

func (r *repository) Apply(ctx context.Context, transformers ...Transformer) error {
	if r.config.DisableQueue {
		changes, err := db.WithTransactionT(r.DB, ctx, 2, false, func(ctx context.Context, transaction *sql.Tx) (*TransformerResult, error) {
			subChanges, applyErr := r.ApplyTransformers(ctx, transaction, transformers...)
			if applyErr != nil {
				return nil, applyErr
			}
			return subChanges, nil
		})

		if err != nil {
			return err
		}
		r.notify.Notify()
		r.notifyChangedApps(changes)
		r.notify.NotifyGitSyncStatus()
		return nil
	} else {
		eCh := r.applyDeferred(ctx, transformers...)
		select {
		case err := <-eCh:
			return err
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

func (r *repository) notifyChangedApps(changes *TransformerResult) {
	var changedAppNames []string
	var seen = make(map[string]bool)
	for _, app := range changes.ChangedApps {
		if _, ok := seen[app.App]; !ok {
			seen[app.App] = true
			changedAppNames = append(changedAppNames, app.App)
		}
	}
	if len(changedAppNames) != 0 {
		r.notify.NotifyChangedApps(changedAppNames)
	}
}

func (r *repository) applyDeferred(ctx context.Context, transformers ...Transformer) <-chan error {
	return r.queue.add(ctx, transformers)
}

func (r *repository) State() *State {
	s, err := r.StateAt()
	if err != nil {
		panic(err)
	}
	return s
}

func (r *repository) StateAt() (*State, error) {
	return &State{
		ReleaseVersionsLimit: r.config.ReleaseVersionsLimit,
		MinorRegexes:         r.config.MinorRegexes,
		MaxNumThreads:        int(r.config.MaxNumThreads),
		DBHandler:            r.DB,
		CloudRunClient:       r.config.CloudRunClient,
	}, nil
}

func (r *repository) Notify() *notify.Notify {
	return &r.notify
}

type State struct {
	ReleaseVersionsLimit uint
	MinorRegexes         []*regexp.Regexp
	MaxNumThreads        int
	// DbHandler will be nil if the DB is disabled
	DBHandler      *db.DBHandler
	CloudRunClient *cloudrun.CloudRunClient
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
	return s.GetEnvironmentLocksFromDB(ctx, transaction, environment)
}

func (s *State) GetEnvironmentApplicationLocks(ctx context.Context, transaction *sql.Tx, environment, application string) (map[string]Lock, error) {
	return s.GetEnvironmentApplicationLocksFromDB(ctx, transaction, environment, application)
}

func (s *State) GetEnvironmentApplicationLocksFromDB(ctx context.Context, transaction *sql.Tx, environment, application string) (map[string]Lock, error) {
	if transaction == nil {
		return nil, fmt.Errorf("GetEnvironmentApplicationLocksFromDB: No transaction provided")
	}
	lockIds, err := s.DBHandler.DBSelectAllAppLocks(ctx, transaction, environment, application)
	if err != nil {
		return nil, err
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

func (s *State) GetEnvironmentTeamLocks(ctx context.Context, transaction *sql.Tx, environment, team string) (map[string]Lock, error) {
	return s.GetEnvironmentTeamLocksFromDB(ctx, transaction, environment, team)
}

func (s *State) GetEnvironmentTeamLocksFromDB(ctx context.Context, transaction *sql.Tx, environment, team string) (map[string]Lock, error) {
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

func (s *State) GetDeploymentMetaData(ctx context.Context, transaction *sql.Tx, environment, application string) (string, time.Time, error) {
	result, err := s.DBHandler.DBSelectLatestDeployment(ctx, transaction, application, environment)
	if err != nil {
		return "", time.Time{}, err
	}
	if result != nil {
		return result.Metadata.DeployedByEmail, result.Created, nil
	}
	return "", time.Time{}, err
}

type SuccessReason int64

const (
	NoReason SuccessReason = iota
	DirDoesNotExist
	DirNotEmpty
)

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

func (s *State) GetQueuedVersionAllAppsFromDB(ctx context.Context, transaction *sql.Tx, environment string) (map[string]*uint64, error) {
	queuedDeployments, err := s.DBHandler.DBSelectLatestDeploymentAttemptOfAllApps(ctx, transaction, environment)
	result := map[string]*uint64{}
	if err != nil || queuedDeployments == nil {
		return result, err
	}
	for _, queuedDeployment := range queuedDeployments {
		var v *uint64
		if queuedDeployment.Version != nil {
			parsedInt := uint64(*queuedDeployment.Version)
			v = &parsedInt
		} else {
			v = nil
		}
		result[queuedDeployment.App] = v
	}
	return result, nil
}

func (s *State) GetQueuedVersion(ctx context.Context, transaction *sql.Tx, environment string, application string) (*uint64, error) {
	return s.GetQueuedVersionFromDB(ctx, transaction, environment, application)
}
func (s *State) GetQueuedVersionOfAllApps(ctx context.Context, transaction *sql.Tx, environment string) (map[string]*uint64, error) {
	return s.GetQueuedVersionAllAppsFromDB(ctx, transaction, environment)
}

func (s *State) DeleteQueuedVersionFromDB(ctx context.Context, transaction *sql.Tx, environment string, application string) error {
	return s.DBHandler.DBDeleteDeploymentAttempt(ctx, transaction, environment, application)
}

func (s *State) DeleteQueuedVersion(ctx context.Context, transaction *sql.Tx, environment string, application string) error {
	return s.DeleteQueuedVersionFromDB(ctx, transaction, environment, application)
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
func (s *State) GetAllLatestDeployments(ctx context.Context, transaction *sql.Tx, environment string, allApps []string) (map[string]*int64, error) {
	return s.DBHandler.DBSelectAllLatestDeploymentsOnEnvironment(ctx, transaction, environment)
}

func (s *State) GetAllLatestReleases(ctx context.Context, transaction *sql.Tx, allApps []string) (map[string][]int64, error) {
	return s.DBHandler.DBSelectAllReleasesOfAllApps(ctx, transaction)
}

func (s *State) GetEnvironmentApplicationVersion(ctx context.Context, transaction *sql.Tx, environment string, application string) (*uint64, error) {
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

func (s *State) GetTeamName(ctx context.Context, transaction *sql.Tx, application string) (string, error) {
	return s.GetApplicationTeamOwner(ctx, transaction, application)
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

func (s *State) GetAllEnvironmentNames(ctx context.Context, transaction *sql.Tx) ([]string, error) {
	dbAllEnvs, err := s.DBHandler.DBSelectAllEnvironments(ctx, transaction)
	if err != nil {
		return nil, fmt.Errorf("unable to retrieve all environments, error: %w", err)
	}
	if dbAllEnvs == nil {
		return []string{}, nil
	}
	return dbAllEnvs, nil
}

func (s *State) GetAllEnvironmentConfigs(ctx context.Context, transaction *sql.Tx) (map[string]config.EnvironmentConfig, error) {
	return s.GetAllEnvironmentConfigsFromDB(ctx, transaction)
}

func (s *State) GetAllDeploymentsForApp(ctx context.Context, transaction *sql.Tx, appName string) (map[string]int64, error) {
	return s.GetAllDeploymentsForAppFromDB(ctx, transaction, appName)
}
func (s *State) GetAllDeploymentsForAppAtTimestamp(ctx context.Context, transaction *sql.Tx, appName string, ts time.Time) (map[string]int64, error) {
	return s.GetAllDeploymentsForAppFromDBAtTimestamp(ctx, transaction, appName, ts)
}

func (s *State) GetAllDeploymentsForAppFromDB(ctx context.Context, transaction *sql.Tx, appName string) (map[string]int64, error) {
	result, err := s.DBHandler.DBSelectAllDeploymentsForApp(ctx, transaction, appName)
	if err != nil {
		return nil, err
	}
	if result == nil {
		return map[string]int64{}, nil
	}
	return result, nil
}

func (s *State) GetAllDeploymentsForAppFromDBAtTimestamp(ctx context.Context, transaction *sql.Tx, appName string, ts time.Time) (map[string]int64, error) {
	result, err := s.DBHandler.DBSelectAllDeploymentsForAppAtTimestamp(ctx, transaction, appName, ts)
	if err != nil {
		return nil, err
	}
	if result == nil {
		return map[string]int64{}, nil
	}
	return result, nil
}

func (s *State) GetAllEnvironmentConfigsFromDB(ctx context.Context, transaction *sql.Tx) (map[string]config.EnvironmentConfig, error) {
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
	ret := make(map[string]config.EnvironmentConfig)
	for _, env := range *envs {
		ret[env.Name] = env.Config
	}
	return ret, nil
}

func (s *State) GetEnvironmentConfig(ctx context.Context, transaction *sql.Tx, environmentName string) (*config.EnvironmentConfig, error) {
	return s.GetEnvironmentConfigFromDB(ctx, transaction, environmentName)
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
	return s.GetEnvironmentApplicationsFromDB(ctx, transaction, environment)
}

func (s *State) GetEnvironmentApplicationsAtTimestamp(ctx context.Context, transaction *sql.Tx, environment string, ts time.Time) ([]string, error) {
	return s.GetEnvironmentApplicationsFromDBAtTimestamp(ctx, transaction, environment, ts)
}

func (s *State) GetEnvironmentApplicationsFromDB(ctx context.Context, transaction *sql.Tx, environment string) ([]string, error) {
	envInfo, err := s.DBHandler.DBSelectEnvironment(ctx, transaction, environment)
	if err != nil {
		return nil, err
	}
	if envInfo == nil {
		return nil, fmt.Errorf("environment %s not found", environment)
	}
	if envInfo.Applications == nil {
		return make([]string, 0), nil
	}
	return envInfo.Applications, nil
}

func (s *State) GetEnvironmentApplicationsFromDBAtTimestamp(ctx context.Context, transaction *sql.Tx, environment string, ts time.Time) ([]string, error) {
	envInfo, err := s.DBHandler.DBSelectEnvironmentAtTimestamp(ctx, transaction, environment, ts)
	if err != nil {
		return nil, err
	}
	if envInfo == nil {
		return nil, fmt.Errorf("environment %s not found", environment)
	}
	if envInfo.Applications == nil {
		return make([]string, 0), nil
	}
	return envInfo.Applications, nil
}

// GetApplications returns all apps that exist in any env
func (s *State) GetApplications(ctx context.Context, transaction *sql.Tx) ([]string, error) {
	applications, err := s.DBHandler.DBSelectAllApplications(ctx, transaction)
	if err != nil {
		return nil, err
	}
	if applications == nil {
		return make([]string, 0), nil
	}
	return applications, nil
}

func (s *State) GetAllApplicationReleases(ctx context.Context, transaction *sql.Tx, application string) ([]uint64, error) {
	releases, err := s.DBHandler.DBSelectAllReleasesOfApp(ctx, transaction, application)
	if err != nil {
		return nil, fmt.Errorf("could not get all releases of app %s: %v", application, err)
	}
	if releases == nil {
		return nil, fmt.Errorf("could not get all releases of app %s (nil)", application)
	}
	res := conversion.ToUint64Slice(releases)
	return res, nil
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
		IsMinor:         rel.IsMinor,
		IsPrepublish:    rel.IsPrepublish,
		Environments:    rel.Environments,
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

func (s *State) IsUndeployVersion(ctx context.Context, transaction *sql.Tx, application string, version uint64) (bool, error) {
	release, err := s.DBHandler.DBSelectReleaseByVersion(ctx, transaction, application, version, true)
	return release.Metadata.UndeployVersion, err
}

func (s *State) GetApplicationReleasesDB(ctx context.Context, transaction *sql.Tx, application string, versions []uint64) ([]*Release, error) {
	var result []*Release
	rels, err := s.DBHandler.DBSelectReleasesByVersions(ctx, transaction, application, versions, true)
	if err != nil {
		return nil, fmt.Errorf("could not get release of app %s: %v", application, err)
	}
	if rels == nil {
		return nil, nil
	}
	for _, rel := range rels {
		r := &Release{
			Version:         rel.ReleaseNumber,
			UndeployVersion: rel.Metadata.UndeployVersion,
			SourceAuthor:    rel.Metadata.SourceAuthor,
			SourceCommitId:  rel.Metadata.SourceCommitId,
			SourceMessage:   rel.Metadata.SourceMessage,
			CreatedAt:       rel.Created,
			DisplayVersion:  rel.Metadata.DisplayVersion,
			IsMinor:         rel.Metadata.IsMinor,
			IsPrepublish:    rel.Metadata.IsPrepublish,
			Environments:    rel.Environments,
		}
		result = append(result, r)
	}
	return result, nil
}

func (s *State) GetApplicationRelease(ctx context.Context, transaction *sql.Tx, application string, version uint64) (*Release, error) {
	env, err := s.DBHandler.DBSelectReleaseByVersion(ctx, transaction, application, version, true)
	if err != nil {
		return nil, fmt.Errorf("could not get release of app %s: %v", application, err)
	}
	if env == nil {
		return nil, nil
	}
	return &Release{
		Version:         env.ReleaseNumber,
		UndeployVersion: env.Metadata.UndeployVersion,
		SourceAuthor:    env.Metadata.SourceAuthor,
		SourceCommitId:  env.Metadata.SourceCommitId,
		SourceMessage:   env.Metadata.SourceMessage,
		CreatedAt:       env.Created,
		DisplayVersion:  env.Metadata.DisplayVersion,
		IsMinor:         env.Metadata.IsMinor,
		IsPrepublish:    env.Metadata.IsPrepublish,
		Environments:    env.Environments,
	}, nil
}

func (s *State) GetApplicationReleaseManifests(ctx context.Context, transaction *sql.Tx, application string, version uint64) (map[string]*api.Manifest, error) {
	manifests := map[string]*api.Manifest{}
	release, err := s.DBHandler.DBSelectReleaseByVersion(ctx, transaction, application, version, true)
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
}

func (s *State) GetApplicationTeamOwner(ctx context.Context, transaction *sql.Tx, application string) (string, error) {
	app, err := s.DBHandler.DBSelectApp(ctx, transaction, application)
	if err != nil {
		return "", fmt.Errorf("could not get team of app %s: %v", application, err)
	}
	if app == nil {
		return "", fmt.Errorf("could not get team of app %s - could not find app", application)
	}
	return app.Metadata.Team, nil
}

func (s *State) GetAllApplicationsTeamOwner(ctx context.Context, transaction *sql.Tx) (map[string]string, error) {
	result := make(map[string]string)
	apps, err := s.DBHandler.DBSelectAllAppsMetadata(ctx, transaction)
	if err != nil {
		return result, fmt.Errorf("could not get team of all apps: %w", err)
	}
	for _, app := range apps {
		result[app.App] = app.Metadata.Team
	}
	return result, nil
}

func (s *State) GetApplicationTeamOwnerAtTimestamp(ctx context.Context, transaction *sql.Tx, application string, ts time.Time) (string, error) {
	app, err := s.DBHandler.DBSelectAppAtTimestamp(ctx, transaction, application, ts)
	if err != nil {
		return "", fmt.Errorf("could not get team of app %s: %v", application, err)
	}
	if app == nil {
		return "", fmt.Errorf("could not get team of app %s - could not find app", application)
	}
	return app.Metadata.Team, nil
}

// ProcessQueue checks if there is something in the queue
// deploys if necessary
// deletes the queue
func (s *State) ProcessQueue(ctx context.Context, transaction *sql.Tx, environment string, application string) (string, error) {
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

func (s *State) ProcessQueueAllApps(ctx context.Context, transaction *sql.Tx, environment string) (string, error) {
	queuedVersions, err := s.GetQueuedVersionOfAllApps(ctx, transaction, environment)
	if err != nil {
		return "", err
	}
	queueDeploymentMessage := ""
	for application, queuedVersion := range queuedVersions {
		if queuedVersion == nil {
			continue
		}

		currentlyDeployedVersion, err := s.GetEnvironmentApplicationVersion(ctx, transaction, environment, application)
		if err != nil {
			return "", err
		}

		if currentlyDeployedVersion != nil && *queuedVersion == *currentlyDeployedVersion {
			err = s.DeleteQueuedVersion(ctx, transaction, environment, application)
			if err != nil {
				return "", err
			}
			if queueDeploymentMessage != "" {
				queueDeploymentMessage += "\n"
			}
			queueDeploymentMessage += fmt.Sprintf("deleted queued version %d because it was already deployed. app=%q env=%q", *queuedVersion, application, environment)
		}
	}
	return queueDeploymentMessage, nil
}
