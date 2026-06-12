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

package cmd

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/DataDog/datadog-go/v5/statsd"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
	"gopkg.in/DataDog/dd-trace-go.v1/ddtrace"
	"gopkg.in/DataDog/dd-trace-go.v1/ddtrace/tracer"

	"github.com/freiheit-com/kuberpult/pkg/api/v1"
	"github.com/freiheit-com/kuberpult/pkg/auth"
	"github.com/freiheit-com/kuberpult/pkg/backoff"
	"github.com/freiheit-com/kuberpult/pkg/config"
	"github.com/freiheit-com/kuberpult/pkg/db"
	"github.com/freiheit-com/kuberpult/pkg/interceptors"
	"github.com/freiheit-com/kuberpult/pkg/logging"
	"github.com/freiheit-com/kuberpult/pkg/setup"
	"github.com/freiheit-com/kuberpult/pkg/types"
	"github.com/freiheit-com/kuberpult/pkg/valid"
	"github.com/freiheit-com/kuberpult/services/manifest-repo-export-service/pkg/argocd"
	"github.com/freiheit-com/kuberpult/services/manifest-repo-export-service/pkg/repository"
	"github.com/freiheit-com/kuberpult/services/manifest-repo-export-service/pkg/service"
)

const (
	minReleaseVersionsLimit = 5
	maxReleaseVersionsLimit = 30

	maxEslProcessingTimeSeconds int64 = 600 // see eslProcessingIdleTimeSeconds in values.yaml
)

func RunServer() {
	logging.Wrap(context.Background(), func(ctx context.Context) error {
		defer logging.HandlePanic(true)
		err := Run(ctx)
		if err != nil {
			logging.Error(ctx, "error in startup.", zap.Error(err))
		}
		return nil
	})
}

func Run(ctx context.Context) error {
	dbLocation, err := valid.ReadEnvVar("KUBERPULT_DB_LOCATION")
	if err != nil {
		return err
	}
	dbName, err := valid.ReadEnvVar("KUBERPULT_DB_NAME")
	if err != nil {
		return err
	}
	dbUserName, err := valid.ReadEnvVar("KUBERPULT_DB_USER_NAME")
	if err != nil {
		return err
	}
	dbPassword, err := valid.ReadEnvVar("KUBERPULT_DB_USER_PASSWORD")
	if err != nil {
		return err
	}
	dbAuthProxyPort, err := valid.ReadEnvVar("KUBERPULT_DB_AUTH_PROXY_PORT")
	if err != nil {
		return err
	}
	dbMaxOpen, err := valid.ReadEnvVarUInt("KUBERPULT_DB_MAX_OPEN_CONNECTIONS")
	if err != nil {
		return err
	}
	dbMaxIdle, err := valid.ReadEnvVarUInt("KUBERPULT_DB_MAX_IDLE_CONNECTIONS")
	if err != nil {
		return err
	}
	sslMode, err := valid.ReadEnvVar("KUBERPULT_DB_SSL_MODE")
	if err != nil {
		return err
	}
	gitUrl, err := valid.ReadEnvVar("KUBERPULT_GIT_URL")
	if err != nil {
		return err
	}
	gitBranch, err := valid.ReadEnvVar("KUBERPULT_GIT_BRANCH")
	if err != nil {
		return err
	}
	gitSshKey, err := valid.ReadEnvVar("KUBERPULT_GIT_SSH_KEY")
	if err != nil {
		return err
	}
	gitSshKnownHosts, err := valid.ReadEnvVar("KUBERPULT_GIT_SSH_KNOWN_HOSTS")
	if err != nil {
		return err
	}

	enableMetricsString, err := valid.ReadEnvVar("KUBERPULT_ENABLE_METRICS")
	if err != nil {
		logging.Info(ctx, "datadog metrics are disabled")
	}
	enableMetrics := enableMetricsString == "true"
	dataDogStatsAddr := "127.0.0.1:8125"
	if enableMetrics {
		dataDogStatsAddrEnv, err := valid.ReadEnvVar("KUBERPULT_DOGSTATSD_ADDR")
		if err != nil {
			logging.Info(ctx, "using default dogStatsAddr.", zap.String("addr", dataDogStatsAddr))
		} else {
			dataDogStatsAddr = dataDogStatsAddrEnv
		}
	}

	enableTracesString, err := valid.ReadEnvVar("KUBERPULT_ENABLE_TRACING")
	if err != nil {
		logging.Info(ctx, "datadog traces are disabled")
	}
	enableTraces := enableTracesString == "true"
	if enableTraces {
		tracer.Start()
		defer tracer.Stop()
	}

	releaseVersionLimitStr, err := valid.ReadEnvVar("KUBERPULT_RELEASE_VERSIONS_LIMIT")
	if err != nil {
		return err
	}

	releaseVersionLimit, err := strconv.ParseUint(releaseVersionLimitStr, 10, 64)
	if err != nil {
		return fmt.Errorf("error converting KUBERPULT_RELEASE_VERSIONS_LIMIT, error: %w", err)
	}

	if err := checkReleaseVersionLimit(uint(releaseVersionLimit)); err != nil {
		return fmt.Errorf("error parsing KUBERPULT_RELEASE_VERSIONS_LIMIT, error: %w", err)
	}

	var eslProcessingIdleTimeSeconds int64
	if val, exists := os.LookupEnv("KUBERPULT_ESL_PROCESSING_BACKOFF"); !exists {
		logging.Info(ctx, "environment variable KUBERPULT_ESL_PROCESSING_BACKOFF is not set, using default backoff of 10 seconds")
		eslProcessingIdleTimeSeconds = 10
	} else {
		eslProcessingIdleTimeSeconds, err = strconv.ParseInt(val, 10, 64)
		if err != nil {
			return fmt.Errorf("error converting KUBERPULT_ESL_PROCESSING_BACKOFF, error: %w", err)
		}
	}
	if eslProcessingIdleTimeSeconds < 1 {
		return fmt.Errorf("error KUBERPULT_ESL_PROCESSING_BACKOFF must be >=1 but was: %v", eslProcessingIdleTimeSeconds)
	}
	if eslProcessingIdleTimeSeconds > maxEslProcessingTimeSeconds {
		return fmt.Errorf("error KUBERPULT_ESL_PROCESSING_BACKOFF must be <=%v but was: %v", maxEslProcessingTimeSeconds, eslProcessingIdleTimeSeconds)
	}
	logging.Info(ctx, "eslProcessingTimeSeconds", zap.Int("eslProcessingTimeSeconds", int(eslProcessingIdleTimeSeconds)))

	networkTimeoutSecondsStr, err := valid.ReadEnvVar("KUBERPULT_NETWORK_TIMEOUT_SECONDS")
	if err != nil {
		return err
	}
	networkTimeoutSeconds, err := strconv.ParseUint(networkTimeoutSecondsStr, 10, 64)
	if err != nil {
		return fmt.Errorf("error parsing KUBERPULT_NETWORK_TIMEOUT_SECONDS, error: %w", err)
	}

	argoCdGenerateFilesString, err := valid.ReadEnvVar("KUBERPULT_ARGO_CD_GENERATE_FILES")
	if err != nil {
		return err
	}
	argoCdGenerateFiles := argoCdGenerateFilesString == "true"

	kuberpultVersionRaw, err := valid.ReadEnvVar("KUBERPULT_VERSION")
	if err != nil {
		return err
	}
	logging.Info(ctx, "startup", zap.String("kuberpultVersion", kuberpultVersionRaw))

	dbMigrationLocation, err := valid.ReadEnvVar("KUBERPULT_DB_MIGRATIONS_LOCATION")
	if err != nil {
		return err
	}

	dbOutdatedDeploymentsCleaningEnabled := valid.ReadEnvVarBoolWithDefault("KUBERPULT_OUTDATED_DEPLOYMENTS_CLEANING_ENABLED", false)

	resetGitSyncStatusEnabled := valid.ReadEnvVarBoolWithDefault("KUBERPULT_RESET_GIT_SYNC_STATUS_ENABLED", false)

	failOnErrorWithGitPushTags, err := valid.ReadEnvVarBool("KUBERPULT_FAIL_ON_ERROR_WITH_GIT_PUSH_TAGS")
	if err != nil {
		return err
	}

	dexMock := valid.ReadEnvVarBoolWithDefault("KUBERPULT_DEX_MOCK", false)
	dexEnabled := valid.ReadEnvVarBoolWithDefault("KUBERPULT_DEX_ENABLED", false)
	dexMockRole := valid.ReadEnvVarWithDefault("KUBERPULT_DEX_MOCK_ROLE", "Developer")
	dexRbacPolicyPath := valid.ReadEnvVarWithDefault("KUBERPULT_DEX_RBAC_POLICY_PATH", "")
	dexRbacTeamPath := valid.ReadEnvVarWithDefault("KUBERPULT_DEX_RBAC_TEAM_PATH", "")
	dexDefaultRoleEnabled := valid.ReadEnvVarBoolWithDefault("KUBERPULT_DEX_DEFAULT_ROLE_ENABLED", false)

	var reader auth.GrpcContextReader
	if dexMock {
		if !dexEnabled {
			logging.Fatal(ctx, "dexEnabled must be true if dexMock is true")
		}
		if dexMockRole == "" {
			logging.Fatal(ctx, "dexMockRole must be set to a role (e.g 'DEVELOPER' because dexEnabled=true")
		}
		reader = &auth.DummyGrpcContextReader{Role: dexMockRole}
	} else {
		reader = &auth.DexGrpcContextReader{DexEnabled: dexEnabled, DexDefaultRoleEnabled: dexDefaultRoleEnabled}
	}
	dexRbacPolicy, err := auth.ReadRbacPolicy(dexEnabled, dexRbacPolicyPath)
	if err != nil {
		logging.Fatal(ctx, "dex.read.error", zap.Error(err))
	}
	dexRbacTeam, err := auth.ReadRbacTeam(dexEnabled, dexRbacTeamPath)
	if err != nil {
		logging.Fatal(ctx, "dex.read.error", zap.Error(err))
	}

	experimentalRolloutWithManifest := valid.ReadEnvVarBoolWithDefault("KUBERPULT_EXPERIMENTAL_ROLLOUT_WITH_MANIFEST_ENABLED", false)
	allArgoProjectNames := argocd.AllArgoProjectNameOverrides{}
	rawStringMapEnvironments := valid.StringMap{}
	rawStringMapAAEnvironments := valid.StringMap{}
	if experimentalRolloutWithManifest {
		logging.Warn(ctx, "experimental feature KUBERPULT_EXPERIMENTAL_ROLLOUT_WITH_MANIFEST_ENABLED enabled")

		rawStringMapEnvironments, err = valid.ReadEnvVarJsonMap("KUBERPULT_EXPERIMENTAL_ROLLOUT_WITH_MANIFEST_ENVIRONMENTS")
		if err != nil {
			return err
		}
		rawStringMapAAEnvironments, err = valid.ReadEnvVarJsonMap("KUBERPULT_EXPERIMENTAL_ROLLOUT_WITH_MANIFEST_AA_ENVIRONMENTS")
		if err != nil {
			return err
		}
	}

	renderOptions := argocd.RenderOptions{}
	renderOptions.RenderApps, err = valid.ReadEnvVarBool("KUBERPULT_RENDERING_RENDER_APPS")
	if err != nil {
		return err
	}
	renderOptions.RenderBrackets, err = valid.ReadEnvVarBool("KUBERPULT_RENDERING_RENDER_BRACKETS")
	if err != nil {
		return err
	}
	renderOptions.PointToBrackets, err = valid.ReadEnvVarBool("KUBERPULT_RENDERING_ROOT_APP_POINTS_TO_BRACKETS")
	if err != nil {
		return err
	}

	renderOptions.RootAppFiltering.Enabled, err = valid.ReadEnvVarBool("KUBERPULT_EXPERIMENTAL_ROOT_APP_FILTER_ENABLED")
	if err != nil {
		return err
	}
	tmp, err := valid.ReadEnvVarAsList("KUBERPULT_EXPERIMENTAL_ROOT_APP_FILTER_ENVIRONMENTS", ",")
	if err != nil {
		return err
	}
	renderOptions.RootAppFiltering.EnabledEnvironments = types.StringsToEnvNames(tmp)
	logging.Info(ctx, "root app filter", zap.Any("filter", renderOptions.RootAppFiltering))

	dbCfg := db.DBConfig{
		DbHost:         dbLocation,
		DbPort:         dbAuthProxyPort,
		DriverName:     "postgres",
		DbName:         dbName,
		DbPassword:     dbPassword,
		DbUser:         dbUserName,
		MigrationsPath: dbMigrationLocation,
		SSLMode:        sslMode,

		MaxIdleConnections: dbMaxIdle,
		MaxOpenConnections: dbMaxOpen,

		DatadogServiceName: "kuberpult-manifest-repo-export-service",
		DatadogEnabled:     enableTraces,
	}
	dbHandler, err := db.Connect(ctx, dbCfg)
	if err != nil {
		return err
	}
	var ddMetrics statsd.ClientInterface
	if enableMetrics {
		ddMetrics, err = statsd.New(dataDogStatsAddr, statsd.WithNamespace("Kuberpult"))
		if err != nil {
			logging.Fatal(ctx, "datadog.metrics.error", zap.Error(err))
		}
	}

	cfg := repository.RepositoryConfig{
		URL:            gitUrl,
		Path:           "./repository",
		TagsPath:       "./repository_tags",
		CommitterEmail: "kuberpult@freiheit.com",
		CommitterName:  "kuberpult",
		Credentials: repository.Credentials{
			SshKey: gitSshKey,
		},
		Certificates: repository.Certificates{
			KnownHostsFile: gitSshKnownHosts,
		},
		Branch:              gitBranch,
		NetworkTimeout:      time.Duration(networkTimeoutSeconds) * time.Second,
		ReleaseVersionLimit: uint(releaseVersionLimit),
		ArgoCdGenerateFiles: argoCdGenerateFiles,

		DBHandler: dbHandler,

		DDMetrics: ddMetrics,

		ArgoRenderOptions: &renderOptions,
		ArgoProjectNames:  &allArgoProjectNames, // note that this is empty here, we'll fill it later
	}
	repo, err := repository.New(ctx, cfg)
	if err != nil {
		return fmt.Errorf("repository.new failed %v", err)
	}

	logging.Info(ctx, "Running SQL Migrations")

	migErr := db.RunDBMigrations(ctx, dbCfg)
	if migErr != nil {
		logging.Fatal(ctx, "Error running database migrations: ", zap.Error(migErr))
	}
	logging.Info(ctx, "Finished with SQL migration.")

	// the custom migrations that we also want to run on startup
	logging.Info(ctx, "Running custom migrations")
	err = dbHandler.RunCustomMigrations(ctx)
	if err != nil {
		logging.Fatal(ctx, "error running custom migrations", zap.Error(err))
	}
	logging.Info(ctx, "Finished custom migrations")

	// the custom migrations that we only want to run on startup if the flag is enabled
	if dbOutdatedDeploymentsCleaningEnabled {
		err := dbHandler.RunCustomMigrationCleanOutdatedDeployments(ctx)
		if err != nil {
			logging.Error(ctx, "error running migrations for cleaning outdated deployments - you can disable this cleaning operation with 'db.outdatedDeploymentsCleaning.enabled:false'", zap.Error(err))
		}
	}

	if resetGitSyncStatusEnabled {
		err := dbHandler.RunCustomMigrationCleanGitSyncStatus(ctx)
		if err != nil {
			logging.Error(ctx, "error cleaning git sync status - you can disable this cleaning operation with 'db.resetGitSyncStatus.enabled:false'", zap.Error(err))
		}
	}

	// we need to run this after the initial migrations are done
	existingEnvsFullName := []types.EnvName{}
	err = dbHandler.WithTransaction(ctx, true, func(ctx context.Context, transaction *sql.Tx) error {
		dbEnvNames, err := dbHandler.DBSelectAllEnvironments(ctx, transaction)
		if err != nil {
			return err
		}
		for _, dbEnvName := range dbEnvNames {
			dbEnv, err := dbHandler.DBSelectEnvironment(ctx, transaction, dbEnvName)
			if err != nil {
				return err
			}
			if config.IsAAEnv(&dbEnv.Config) {
				cfgs := dbEnv.Config.ArgoCdConfigs.ArgoCdConfigurations
				prefix := ""
				if dbEnv.Config.ArgoCdConfigs.CommonEnvPrefix != nil {
					prefix = *dbEnv.Config.ArgoCdConfigs.CommonEnvPrefix
				}
				for _, cfg := range cfgs {
					concreteEnvName := cfg.ConcreteEnvName
					validEnvName := types.EnvName(prefix + "-" + string(dbEnvName) + "-" + concreteEnvName)
					existingEnvsFullName = append(existingEnvsFullName, validEnvName)
				}
			} else {
				existingEnvsFullName = append(existingEnvsFullName, dbEnvName)
			}
		}
		return nil
	})
	if err != nil {
		return err
	}
	allArgoProjectNames.ActiveActiveEnvironments = ParseEnvironmentOverrides(ctx, rawStringMapAAEnvironments, existingEnvsFullName)
	allArgoProjectNames.Environments = ParseEnvironmentOverrides(ctx, rawStringMapEnvironments, existingEnvsFullName)

	grpcStreamInterceptors := []grpc.StreamServerInterceptor{
		func(srv any, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
			defer logging.HandlePanic(true)
			return handler(srv, ss)
		},
	}
	grpcUnaryInterceptors := []grpc.UnaryServerInterceptor{
		func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp any, err error) {
			defer logging.HandlePanic(true)
			return handler(ctx, req)
		},
		func(ctx context.Context,
			req interface{},
			info *grpc.UnaryServerInfo,
			handler grpc.UnaryHandler) (interface{}, error) {
			return interceptors.UnaryUserContextInterceptor(ctx, req, info, handler, reader)
		},
	}

	shutdownCh := make(chan struct{})
	setup.Run(ctx, setup.ServerConfig{
		HTTP: []setup.HTTPConfig{
			{
				BasicAuth: nil,
				Shutdown:  nil,
				Port:      "8080",
				Register:  nil,
			},
		},
		GRPC: &setup.GRPCConfig{
			Shutdown: nil,
			Port:     "8443",
			Opts: []grpc.ServerOption{
				grpc.ChainStreamInterceptor(grpcStreamInterceptors...),
				grpc.ChainUnaryInterceptor(grpcUnaryInterceptors...),
			},
			Register: func(srv *grpc.Server) {
				api.RegisterManifestExportGitServiceServer(srv, &service.GitServer{
					Repository: repo,
					Config:     cfg,
					PageSize:   10,
					DBHandler:  dbHandler,
					RBACConfig: auth.RBACConfig{
						DexEnabled: dexEnabled,
						Policy:     dexRbacPolicy,
						Team:       dexRbacTeam,
					},
				})
				reflection.Register(srv)
			},
		},
		Background: []setup.BackgroundTaskConfig{
			{
				Shutdown: nil,
				Name:     "processEsls",
				Run: func(ctx context.Context, reporter *setup.HealthReporter) error {
					reporter.ReportReady("Processing Esls")
					// TODO(Task 6): read maxBatchSize from KUBERPULT_MAX_EXPORT_BATCH_SIZE. A batch
					// size of 1 reproduces exactly today's single-event behavior (safe default).
					maxBatchSize := 1
					return processEsls(ctx, repo, dbHandler, cfg.DDMetrics, eslProcessingIdleTimeSeconds, failOnErrorWithGitPushTags, maxBatchSize)
				},
			},
		},
		Shutdown: func(ctx context.Context) error {
			close(shutdownCh)
			return nil
		},
	})
	return nil
}

func ParseEnvironmentOverrides(ctx context.Context, configuredArgoNamesPerEnv valid.StringMap, existingEnvsInDb []types.EnvName) *argocd.ArgoProjectNamesPerEnv {
	// temporarily put envs into a map for easier access:
	existingEnvsInDbMap := map[types.EnvName]struct{}{}
	for _, e := range existingEnvsInDb {
		existingEnvsInDbMap[e] = struct{}{}
	}

	// check that all envs exist:
	result := argocd.ArgoProjectNamesPerEnv{}
	for environment, argoProjectName := range configuredArgoNamesPerEnv {
		env := types.EnvName(environment)
		_, exists := existingEnvsInDbMap[env]
		if exists {
			result[env] = types.ArgoProjectName(argoProjectName)
		} else {
			logging.Error(ctx, "overridden environment does not exist - continuing with default argoProject name",
				zap.String("env", environment),
				zap.String("existingEnvs", fmt.Sprintf("%v+", existingEnvsInDbMap)),
				zap.String("helm-parameter", "manifestRepoExport.experimentalRolloutWithManifest"),
				zap.String("experimental", "will continue anyway, because this feature is experimental"),
			)
		}
	}
	return &result
}

func processEsls(
	ctx context.Context,
	repo repository.Repository,
	dbHandler *db.DBHandler,
	ddMetrics statsd.ClientInterface,
	eslProcessingIdleTimeSeconds int64,
	failOnErrorWithGitPushTags bool,
	maxBatchSize int,
) error {
	var sleepDuration = backoff.MakeSimpleBackoff(
		time.Second*time.Duration(eslProcessingIdleTimeSeconds),
		time.Second*time.Duration(maxEslProcessingTimeSeconds),
	)
	for {
		span, ctxOneEvent := tracer.StartSpanFromContext(ctx, "processOneEvent")
		wantedSleepTime, err := ProcessOneEvent(ctxOneEvent, repo, dbHandler, ddMetrics, &sleepDuration, failOnErrorWithGitPushTags, maxBatchSize)
		span.Finish(tracer.WithError(err))
		if err != nil {
			return err
		}
		if wantedSleepTime > 0 {
			measureDelays(ctx, ddMetrics, 0, 0)
			time.Sleep(wantedSleepTime)
		}
	}
}

func ProcessOneEvent(
	ctx context.Context,
	repo repository.Repository,
	dbHandler *db.DBHandler,
	ddMetrics statsd.ClientInterface,
	sleepDuration *backoff.SimpleBackoff,
	failOnErrorWithGitPushTags bool,
	maxBatchSize int,
) (
	time.Duration,
	error,
) {
	const transactionRetries = 10
	var transformers []repository.Transformer
	var esls []*db.EslEventRow
	var commitHashes []string // aligned one-to-one with esls; "" where an event produced no commit
	const readonly = true     // we just handle the reading here, there's another transaction for writing the result to the db/git

	// Capture the current HEAD before applying anything. We use it for two purposes:
	//   1. to reset the branch back to this commit at the start of every apply attempt (see below);
	//   2. KUBERPULT_MINIMIZE_GIT_DATA: NoOp events create no commit, so per-commit timestamps are //nolint:misspell
	//      only written for events that actually produced one.
	oldCommitId, err := repo.GetHeadCommitId()
	if err != nil {
		d := sleepDuration.NextBackOff()
		if sleepDuration.IsAtMax() {
			return 0, err
		}
		logging.Info(ctx, "error getting current commid ID, will try again.", zap.Error(err))
		return d, nil
	}
	err = dbHandler.WithTransactionR(ctx, transactionRetries, readonly, func(ctx context.Context, transaction *sql.Tx) error {
		// Git commits are NOT rolled back when this read transaction retries, so a retry would
		// re-apply on top of the previous attempt's commits and stack duplicates (R-1). Resetting to
		// the pre-batch HEAD at the top of EVERY attempt makes the apply idempotent across retries.
		// This must be inside the closure (WithTransactionR retries by re-invoking it), not before it.
		if resetErr := repo.ResetHardTo(ctx, oldCommitId); resetErr != nil {
			return resetErr
		}
		var err2 error
		transformers, esls, commitHashes, err2 = HandleOneTransformer(ctx, transaction, dbHandler, ddMetrics, repo, maxBatchSize)
		return err2
	})
	if err != nil {
		if len(esls) == 0 {
			logging.Error(ctx, "skipping esl event, because we could not construct esl object.", zap.Error(err))
			return 0, err
		}
		logging.Error(ctx, "skipping esl event, because it returned an error.", zap.Error(err))
		// after this many tries, we can just skip it.
		// TODO(Task 5): on a batch error, fall back to processing the batch one event at a time so
		// only the offending event is failed and the preceding good events still commit. Until then
		// we fail the first event of the batch (correct while maxBatchSize == 1).
		err2 := handleFailedEvent(ctx, dbHandler, transactionRetries, esls[0], err.Error())
		if err2 != nil {
			return 0, fmt.Errorf("error in DBWriteFailedEslEvent %v", err2)
		}
		sleepDuration.Reset()
	} else {
		if len(transformers) == 0 {
			sleepDuration.Reset()
			d := sleepDuration.NextBackOff()
			measureGitPushFailures(ctx, ddMetrics, false)
			logging.Info(ctx, "event processing skipped, will try again.")
			return d, nil
		}
		// The cutoff is a single cursor; writing the highest version of the batch marks the whole
		// batch as processed.
		highestEsl := esls[len(esls)-1]
		logging.Info(ctx, "event processed successfully, now writing to cutoff and pushing...")
		err = dbHandler.WithTransactionR(ctx, 2, false, func(ctx context.Context, transaction *sql.Tx) error {
			err2 := db.DBWriteCutoff(dbHandler, ctx, transaction, highestEsl.EslVersion)
			if err2 != nil {
				return err2
			}
			err2 = repo.PushRepo(ctx)
			if err2 != nil {
				d := sleepDuration.NextBackOff()
				logging.Info(ctx, "error pushing, will try again.", zap.Error(err2))
				measureGitPushFailures(ctx, ddMetrics, true)
				time.Sleep(d)
				return err2
			} else {
				measureGitPushFailures(ctx, ddMetrics, false)
			}

			// A batch produces a chain of <= N commits. Write one commit-transaction-timestamp per
			// commit, using that event's own Created time. Events that produced no commit (NoOp) have
			// an empty hash and are skipped (R-3/R-4). Head-only is not viable: intermediate commits
			// are looked up by hash elsewhere and would otherwise have no timestamp row.
			anyCommit := false
			for i := range commitHashes {
				if commitHashes[i] == "" {
					continue
				}
				anyCommit = true
				if err3 := dbHandler.DBWriteCommitTransactionTimestamp(ctx, transaction, commitHashes[i], esls[i].Created); err3 != nil {
					return err3
				}
			}

			if !anyCommit {
				logging.Warn(ctx, "no commit was created, tagging skipped")
				return nil
			}
			// Push each transformer's git tag (R-11). The export's CreateApplicationVersion carries
			// no tag today, so this loop is a no-op for batched releases, but we loop per transformer
			// (not just the last) so release-level tags work if they are ever wired in.
			for i := range transformers {
				gitTag := transformers[i].GetGitTag()
				if gitTag == "" {
					continue
				}
				pushErr := HandleGitTagPush(ctx, repo, gitTag, ddMetrics, failOnErrorWithGitPushTags)
				if pushErr != nil {
					return handleFailedEvent(ctx, dbHandler, transactionRetries, esls[i],
						fmt.Sprintf("error while pushing the git tag '%s': %v", gitTag, pushErr.Error()))
				}
			}
			return nil
		})
		if err != nil {
			// If we fail to push to repo or to update the cutoff, we say that SYNC has failed. Each
			// event's unsynced apps are keyed under its own transformer id, so we must loop over every
			// esl version in the batch (not just the highest).
			err = dbHandler.WithTransactionR(ctx, 2, false, func(ctx context.Context, transaction *sql.Tx) error {
				for _, esl := range esls {
					if e := dbHandler.DBBulkUpdateUnsyncedApps(ctx, transaction, db.TransformerID(esl.EslVersion), db.SYNC_FAILED); e != nil {
						return e
					}
				}
				return nil
			})
			logging.Error(ctx, "error updating state for ui.", zap.Error(err))
			err3 := repo.FetchAndReset(ctx)
			if err3 != nil {
				d := sleepDuration.NextBackOff()
				logging.Info(ctx, "error fetching repo, will try again.")
				return d, nil
			}
		} else {
			// After a successful push, mark all batched events' apps SYNCED. Loop per esl version for
			// the same reason as the SYNC_FAILED path above.
			err = dbHandler.WithTransactionR(ctx, 2, false, func(ctx context.Context, transaction *sql.Tx) error {
				for _, esl := range esls {
					if e := dbHandler.DBBulkUpdateUnsyncedApps(ctx, transaction, db.TransformerID(esl.EslVersion), db.SYNCED); e != nil {
						return e
					}
				}
				return nil
			})
			if err != nil {
				logging.Error(ctx, "Failed writing sync status after successful operation! Repo has been updated, but sync status has not.", zap.Error(err))
			}
		}
	}
	repo.Notify().Notify() // Notify git sync status

	err = repository.MeasureGitSyncStatus(ctx, ddMetrics, dbHandler)
	if err != nil {
		logging.Error(ctx, "Failed sending git sync status metrics.", zap.Error(err))
		// if just the metrics fail, we don't want to exit with an error
	}
	return 0, nil
}

func handleFailedEvent(ctx context.Context, dbHandler *db.DBHandler, transactionRetries uint8, esl *db.EslEventRow, reason string) error {
	err := dbHandler.WithTransactionR(ctx, transactionRetries, false, func(ctx context.Context, transaction *sql.Tx) error {
		err2 := dbHandler.DBInsertNewFailedESLEvent(ctx, transaction, &db.EslFailedEventRow{
			EslVersion:            0, // This is overwritten by the DB
			Created:               esl.Created,
			EventType:             esl.EventType,
			EventJson:             esl.EventJson,
			Reason:                reason,
			TransformerEslVersion: esl.EslVersion,
		})
		if err2 != nil {
			return err2
		}

		//If we fail to process the transformer, we say that SYNC has failed
		err2 = dbHandler.DBBulkUpdateUnsyncedApps(ctx, transaction, db.TransformerID(esl.EslVersion), db.SYNC_FAILED)
		if err2 != nil {
			return err2
		}

		return db.DBWriteCutoff(dbHandler, ctx, transaction, esl.EslVersion)
	})
	return err
}

func HandleGitTagPush(ctx context.Context, repo repository.Repository, gitTag types.GitTag, ddMetrics statsd.ClientInterface, failOnErrorWithGitTags bool) error {
	gitTagErr := repo.PushTag(ctx, gitTag)
	if gitTagErr != nil {
		measureGitTagPushFailures(ctx, ddMetrics, gitTag)
	}

	if failOnErrorWithGitTags {
		return gitTagErr
	} else {
		// We just continue as if nothing happened.
		return nil
	}
}

func measureGitTagPushFailures(ctx context.Context, ddMetrics statsd.ClientInterface, gitTag types.GitTag) {
	if ddMetrics != nil {
		metricName := "manifest_export_tag_push_failures"
		err := ddMetrics.Gauge(metricName, 1, []string{"kuberpult_tag_name", string(gitTag)}, 1)
		if err != nil {
			logging.Error(ctx, "datadog_metrics_error", zap.Error(err), zap.String("metricName", metricName))
		}
	}
}

func measureGitPushFailures(ctx context.Context, ddMetrics statsd.ClientInterface, failure bool) {
	if ddMetrics != nil {
		var value float64 = 0
		if failure {
			value = 1
		}
		if err := ddMetrics.Gauge("manifest_export_push_failures", value, []string{}, 1); err != nil {
			logging.Error(ctx, "Error in ddMetrics.Gauge.", zap.Error(err))
		}
	}
}

func measureDelays(ctx context.Context, ddMetrics statsd.ClientInterface, delaySeconds float64, delayEvents uint64) {
	if ddMetrics != nil {
		if err := ddMetrics.Gauge("process_delay_seconds", delaySeconds, []string{}, 1); err != nil {
			logging.Error(ctx, "Error in ddMetrics.Gauge for delay seconds.", zap.Error(err))
		}
		if err := ddMetrics.Gauge("process_delay_events", float64(delayEvents), []string{}, 1); err != nil {
			logging.Error(ctx, "Error in ddMetrics.Gauge for delay events.", zap.Error(err))
		}
	}
}
// HandleOneTransformer reads the next batch of esl events to process (a contiguous run of
// CreateApplicationVersion events, or a single event of any other type — see selectBatch), builds a
// transformer for each, and applies the whole batch in a single repo.Apply. It returns the built
// transformers together with the esl rows of the batch they came from, so the caller can write one
// push + one cutoff for the batch. The returned esl rows are also populated on error so the caller
// can react to the failure (e.g. mark it failed). A batch of size 1 reproduces today's behavior.
// HandleOneTransformer additionally returns, aligned one-to-one with the returned esl rows, the
// hash of the commit each event produced ("" for a NoOp event that produced none), so the caller can
// write one commit-transaction-timestamp per commit.
func HandleOneTransformer(ctx context.Context, transaction *sql.Tx, dbHandler *db.DBHandler, ddMetrics statsd.ClientInterface, repo repository.Repository, maxBatchSize int) ([]repository.Transformer, []*db.EslEventRow, []string, error) {
	if ddMetrics != nil {
		delaySeconds, delayEvents, err := dbHandler.GetCurrentDelays(ctx, transaction)
		if err != nil {
			return nil, nil, nil, fmt.Errorf("error in GetCurrentDelays: %v", err)
		}
		measureDelays(ctx, ddMetrics, delaySeconds, delayEvents)
	}
	eslVersion, err := db.DBReadCutoff(dbHandler, ctx, transaction)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("error in DBReadCutoff: %v", err)
	}
	if eslVersion == nil {
		logging.Info(ctx, "did not find cutoff")
	}
	events, err := dbHandler.DBReadEslEventBatch(ctx, transaction, eslVersion, maxBatchSize)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("error in readEslEventBatch %v", err)
	}
	if len(events) == 0 {
		// no event found
		return nil, nil, nil, nil
	}
	batch := selectBatch(events, maxBatchSize)
	transformers, commitHashes, err := processEslEventBatch(ctx, repo, batch, transaction)
	return transformers, batch, commitHashes, err
}

// buildTransformer constructs the concrete transformer for a single esl event: it resolves the
// transformer type from the event type, unmarshals the event JSON into it, and sets the per-event
// EslVersion and CreationTimestamp. It does NOT apply the transformer, so it is a pure function of
// the esl row and can be unit tested without a DB or git repo. Returns (nil, nil) if the event type
// has no associated transformer.
func buildTransformer(ctx context.Context, esl *db.EslEventRow) (repository.Transformer, error) {
	if esl == nil {
		return nil, fmt.Errorf("esl event nil")
	}
	t, err := getTransformer(ctx, esl.EventType)
	if err != nil {
		return nil, fmt.Errorf("get transformer error %v", err)
	}
	if t == nil {
		// no error, but also no transformer to process:
		return nil, nil
	}
	err = json.Unmarshal(([]byte)(esl.EventJson), &t)
	if err != nil {
		return nil, err
	}
	t.SetEslVersion(db.TransformerID(esl.EslVersion))
	t.SetCreationTimestamp(esl.Created)
	return t, nil
}

// processEslEventBatch builds the transformers for every event in the batch and applies them in
// order via a single repo.Apply call. repo.Apply is variadic and builds the commit chain across the
// transformers (each commits on top of the previous), so a batch of N produces a chain of <= N
// commits (a NoOp CreateApplicationVersion produces none) that a single later push ships. Per-event
// trace/span links are preserved by opening a linked span per event.
// It returns the built transformers and, aligned one-to-one with them (and therefore with the batch
// events), the hash of the commit each transformer produced ("" for a NoOp that produced none).
func processEslEventBatch(ctx context.Context, repo repository.Repository, batch []*db.EslEventRow, tx *sql.Tx) ([]repository.Transformer, []string, error) {
	transformers := make([]repository.Transformer, 0, len(batch))
	for _, esl := range batch {
		t, err := buildTransformer(ctx, esl)
		if err != nil {
			return nil, nil, err
		}
		if t == nil {
			// Every known event type has a transformer; a nil here is unexpected and would break
			// the one-to-one alignment between events, transformers and commit hashes.
			return nil, nil, fmt.Errorf("no transformer for event type %q at eslVersion %d", esl.EventType, esl.EslVersion)
		}
		// Preserve the per-event trace/span link from the originating service.
		var span tracer.Span
		if esl.TraceId != nil && esl.SpanId != nil {
			links := []ddtrace.SpanLink{{TraceID: *esl.TraceId, SpanID: *esl.SpanId}}
			span, _ = tracer.StartSpanFromContext(ctx, "processEslEvent", tracer.WithSpanLinks(links))
		} else {
			span, _ = tracer.StartSpanFromContext(ctx, "processEslEvent")
		}
		span.Finish()
		transformers = append(transformers, t)
	}
	if len(transformers) == 0 {
		return nil, nil, nil
	}
	commitHashes, err := repo.ApplyWithCommitIds(ctx, tx, transformers...)
	if err != nil {
		return nil, nil, fmt.Errorf("error while running repo apply: %v", err)
	}
	return transformers, commitHashes, nil
}

// getTransformer returns an empty transformer of the type according to esl.EventType
func getTransformer(_ context.Context, eslEventType db.EventType) (repository.Transformer, error) {
	switch eslEventType {
	case db.EvtDeployApplicationVersion:
		//exhaustruct:ignore
		return &repository.DeployApplicationVersion{}, nil
	case db.EvtCreateEnvironmentLock:
		//exhaustruct:ignore
		return &repository.CreateEnvironmentLock{}, nil
	case db.EvtDeleteEnvironmentLock:
		//exhaustruct:ignore
		return &repository.DeleteEnvironmentLock{}, nil
	case db.EvtCreateEnvironmentApplicationLock:
		//exhaustruct:ignore
		return &repository.CreateEnvironmentApplicationLock{}, nil
	case db.EvtDeleteEnvironmentApplicationLock:
		//exhaustruct:ignore
		return &repository.DeleteEnvironmentApplicationLock{}, nil
	case db.EvtCreateEnvironmentTeamLock:
		//exhaustruct:ignore
		return &repository.CreateEnvironmentTeamLock{}, nil
	case db.EvtDeleteEnvironmentTeamLock:
		//exhaustruct:ignore
		return &repository.DeleteEnvironmentTeamLock{}, nil
	case db.EvtCreateApplicationVersion:
		//exhaustruct:ignore
		return &repository.CreateApplicationVersion{}, nil
	case db.EvtReleaseTrain:
		//exhaustruct:ignore
		return &repository.ReleaseTrain{}, nil
	case db.EvtCreateEnvironment:
		//exhaustruct:ignore
		return &repository.CreateEnvironment{}, nil
	case db.EvtMigrationTransformer:
		//exhaustruct:ignore
		return &repository.MigrationTransformer{}, nil
	case db.EvtDeleteEnvFromApp:
		//exhaustruct:ignore
		return &repository.DeleteEnvFromApp{}, nil
	case db.EvtCreateUndeployApplicationVersion:
		//exhaustruct:ignore
		return &repository.CreateUndeployApplicationVersion{}, nil
	case db.EvtCreateEnvironmentGroupLock:
		//exhaustruct:ignore
		return &repository.CreateEnvironmentGroupLock{}, nil
	case db.EvtDeleteEnvironmentGroupLock:
		//exhaustruct:ignore
		return &repository.DeleteEnvironmentGroupLock{}, nil
	case db.EvtUndeployApplication:
		//exhaustruct:ignore
		return &repository.UndeployApplication{}, nil
	case db.EvtDeleteEnvironment:
		//exhaustruct:ignore
		return &repository.DeleteEnvironment{}, nil
	case db.EvtExtendAAEnvironment:
		//exhaustruct:ignore
		return &repository.ExtendAAEnvironment{}, nil
	case db.EvtDeleteAAEnvironmentConfig:
		//exhaustruct:ignore
		return &repository.DeleteAAEnvironmentConfig{}, nil
	case db.EvtRenderEnvironment:
		//exhaustruct:ignore
		return &repository.RenderEnvironment{}, nil
	}
	return nil, fmt.Errorf("could not find transformer for event type %v", eslEventType)
}

// selectBatch returns the contiguous prefix of events that should be processed together with a
// single push. The input must be the events as returned by DBReadEslEventBatch: all events with
// eslVersion > cutoff, in ascending eslVersion order, with NO event_type filter applied (see the
// note on DBReadEslEventBatch). This is what lets the returned slice stay strictly contiguous in
// eslVersion — we never skip over an interleaved non-matching event.
//
// The rule:
//   - empty input -> empty batch.
//   - if the first unprocessed event is not a CreateApplicationVersion -> batch of exactly 1
//     (today's single-event behavior, unchanged).
//   - if the first unprocessed event is a CreateApplicationVersion -> the maximal contiguous prefix
//     of CreateApplicationVersion events, capped at maxBatchSize. We stop at the first event of any
//     other type.
//
// maxBatchSize < 1 is treated as 1, so a batch size of 0/1/negative reproduces today's behavior and
// acts as a safe rollback switch.
func selectBatch(events []*db.EslEventRow, maxBatchSize int) []*db.EslEventRow {
	if maxBatchSize < 1 {
		maxBatchSize = 1
	}
	if len(events) == 0 {
		return events
	}
	if events[0].EventType != db.EvtCreateApplicationVersion {
		return events[:1]
	}
	n := 0
	for n < len(events) && n < maxBatchSize && events[n].EventType == db.EvtCreateApplicationVersion {
		n++
	}
	return events[:n]
}

func checkReleaseVersionLimit(limit uint) error {
	if limit < minReleaseVersionsLimit || limit > maxReleaseVersionsLimit {
		return releaseVersionsLimitError{limit: limit}
	}
	return nil
}
