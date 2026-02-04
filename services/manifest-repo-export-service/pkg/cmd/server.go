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
	"errors"
	"strconv"
	"time"

	"github.com/freiheit-com/kuberpult/pkg/backoff"
	"github.com/freiheit-com/kuberpult/pkg/migrations"
	"github.com/freiheit-com/kuberpult/pkg/types"
	"github.com/freiheit-com/kuberpult/pkg/valid"

	"encoding/json"
	"fmt"
	"os"

	"github.com/DataDog/datadog-agent/pkg/trace/log"
	"github.com/DataDog/datadog-go/v5/statsd"
	"github.com/freiheit-com/kuberpult/pkg/api/v1"
	"github.com/freiheit-com/kuberpult/pkg/db"
	"github.com/freiheit-com/kuberpult/pkg/logger"
	"github.com/freiheit-com/kuberpult/pkg/setup"
	"github.com/freiheit-com/kuberpult/services/manifest-repo-export-service/pkg/repository"
	"github.com/freiheit-com/kuberpult/services/manifest-repo-export-service/pkg/service"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
	"gopkg.in/DataDog/dd-trace-go.v1/ddtrace/tracer"
)

const (
	minReleaseVersionsLimit = 5
	maxReleaseVersionsLimit = 30

	maxEslProcessingTimeSeconds int64 = 600 // see eslProcessingIdleTimeSeconds in values.yaml
)

func RunServer() {
	_ = logger.Wrap(context.Background(), func(ctx context.Context) error {
		err := Run(ctx)
		if err != nil {
			logger.FromContext(ctx).Sugar().Errorf("error in startup: %v %#v", err, err)
			err2 := logger.FromContext(ctx).Sync()
			if err2 != nil {
				panic(errors.Join(err, err2))
			}
		}
		return nil
	})
}

func Run(ctx context.Context) error {
	log := logger.FromContext(ctx).Sugar()

	logger.FromContext(ctx).Info("Startup")

	dbLocation, err := valid.ReadEnvVar("KUBERPULT_DB_LOCATION")
	if err != nil {
		return err
	}
	dbName, err := valid.ReadEnvVar("KUBERPULT_DB_NAME")
	if err != nil {
		return err
	}
	dbOption, err := valid.ReadEnvVar("KUBERPULT_DB_OPTION")
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
		log.Info("datadog metrics are disabled")
	}
	enableMetrics := enableMetricsString == "true"
	dataDogStatsAddr := "127.0.0.1:8125"
	if enableMetrics {
		dataDogStatsAddrEnv, err := valid.ReadEnvVar("KUBERPULT_DOGSTATSD_ADDR")
		if err != nil {
			log.Infof("using default dogStatsAddr: %s", dataDogStatsAddr)
		} else {
			dataDogStatsAddr = dataDogStatsAddrEnv
		}
	}

	enableTracesString, err := valid.ReadEnvVar("KUBERPULT_ENABLE_TRACING")
	if err != nil {
		log.Info("datadog traces are disabled")
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
	checkCustomMigrationsString, err := valid.ReadEnvVar("KUBERPULT_CHECK_CUSTOM_MIGRATIONS")
	if err != nil {
		log.Info("datadog metrics are disabled")
	}
	checkCustomMigrations := checkCustomMigrationsString == "true"
	minimizeExportedData, err := valid.ReadEnvVarBool("KUBERPULT_MINIMIZE_EXPORTED_DATA")
	if err != nil {
		return err
	}

	var eslProcessingIdleTimeSeconds int64
	if val, exists := os.LookupEnv("KUBERPULT_ESL_PROCESSING_BACKOFF"); !exists {
		log.Infof("environment variable KUBERPULT_ESL_PROCESSING_BACKOFF is not set, using default backoff of 10 seconds")
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
	log.Infof("eslProcessingTimeSeconds: %d", eslProcessingIdleTimeSeconds)

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
	logger.FromContext(ctx).Info("startup", zap.String("kuberpultVersion", kuberpultVersionRaw))

	dbMigrationLocation, err := valid.ReadEnvVar("KUBERPULT_DB_MIGRATIONS_LOCATION")
	if err != nil {
		return err
	}
	gitTimestampMigrationEnabledString, err := valid.ReadEnvVar("KUBERPULT_GIT_TIMESTAMP_MIGRATIONS_ENABLED")
	if err != nil {
		return err
	}
	dbGitTimestampMigrationEnabled := gitTimestampMigrationEnabledString == "true"

	failOnErrorWithGitPushTags, err := valid.ReadEnvVarBool("KUBERPULT_FAIL_ON_ERROR_WITH_GIT_PUSH_TAGS")
	if err != nil {
		return err
	}

	var dbCfg db.DBConfig
	if dbOption == "postgreSQL" {
		dbCfg = db.DBConfig{
			DbHost:         dbLocation,
			DbPort:         dbAuthProxyPort,
			DriverName:     "postgres",
			DbName:         dbName,
			DbPassword:     dbPassword,
			DbUser:         dbUserName,
			MigrationsPath: dbMigrationLocation,
			WriteEslOnly:   false,
			SSLMode:        sslMode,

			MaxIdleConnections: dbMaxIdle,
			MaxOpenConnections: dbMaxOpen,

			DatadogServiceName: "kuberpult-manifest-repo-export-service",
			DatadogEnabled:     enableTraces,
		}
	} else {
		logger.FromContext(ctx).Fatal("Cannot start without DB configuration was provided.")
	}
	dbHandler, err := db.Connect(ctx, dbCfg)
	if err != nil {
		return err
	}
	var ddMetrics statsd.ClientInterface
	if enableMetrics {
		ddMetrics, err = statsd.New(dataDogStatsAddr, statsd.WithNamespace("Kuberpult"))
		if err != nil {
			logger.FromContext(ctx).Fatal("datadog.metrics.error", zap.Error(err))
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
		Branch:               gitBranch,
		NetworkTimeout:       time.Duration(networkTimeoutSeconds) * time.Second,
		ReleaseVersionLimit:  uint(releaseVersionLimit),
		ArgoCdGenerateFiles:  argoCdGenerateFiles,
		MinimizeExportedData: minimizeExportedData,

		DBHandler: dbHandler,

		DDMetrics: ddMetrics,
	}
	repo, err := repository.New(ctx, cfg)
	if err != nil {
		return fmt.Errorf("repository.new failed %v", err)
	}

	log.Infof("Running SQL Migrations")

	migErr := db.RunDBMigrations(ctx, dbCfg)
	if migErr != nil {
		logger.FromContext(ctx).Fatal("Error running database migrations: ", zap.Error(migErr))
	}
	logger.FromContext(ctx).Info("Finished with basic database migration.")
	kuberpultVersion, err := migrations.ParseKuberpultVersion(kuberpultVersionRaw)
	if err != nil {
		return err
	}
	migrationServer := &service.MigrationServer{
		KuberpultVersion: kuberpultVersion,
		DBHandler:        dbHandler,
		Migrations:       getAllMigrations(dbHandler, repo),
	}
	if shouldRunCustomMigrations(checkCustomMigrations, minimizeExportedData) {
		log.Infof("Running Custom Migrations")

		_, err = migrationServer.EnsureCustomMigrationApplied(ctx, &api.EnsureCustomMigrationAppliedRequest{
			Version: kuberpultVersion,
		})
		if err != nil {
			return fmt.Errorf("error running custom migrations: %w", err)
		}
		log.Infof("Finished Custom Migrations successfully")
	} else {
		logger.FromContext(ctx).Sugar().Infof("Custom Migrations skipped. Kuberpult only runs custom Migrations if " +
			"KUBERPULT_MINIMIZE_EXPORTED_DATA=false and KUBERPULT_CHECK_CUSTOM_MIGRATIONS=true.")
	}
	if dbGitTimestampMigrationEnabled {
		err := dbHandler.RunCustomMigrationReleasesTimestamp(ctx, repo.State().GetAppsAndTeams, repo.State().FixReleasesTimestamp)
		if err != nil {
			return fmt.Errorf("error running migrations for fixing releases timestamp: %w", err)
		}
		err = repo.FixCommitsTimestamp(ctx, *repo.State())
		if err != nil {
			return fmt.Errorf("error fixing commit timestamps: %w", err)
		}
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
			Opts:     []grpc.ServerOption{},
			Register: func(srv *grpc.Server) {
				api.RegisterVersionServiceServer(srv, &service.VersionServiceServer{Repository: repo})
				api.RegisterManifestExportGitServiceServer(srv, &service.GitServer{Repository: repo, Config: cfg, PageSize: 10, DBHandler: dbHandler})
				api.RegisterMigrationServiceServer(srv, migrationServer)
				reflection.Register(srv)
			},
		},
		Background: []setup.BackgroundTaskConfig{
			{
				Shutdown: nil,
				Name:     "processEsls",
				Run: func(ctx context.Context, reporter *setup.HealthReporter) error {
					reporter.ReportReady("Processing Esls")
					return processEsls(ctx, repo, dbHandler, cfg.DDMetrics, eslProcessingIdleTimeSeconds, failOnErrorWithGitPushTags)
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

func getAllMigrations(dbHandler *db.DBHandler, repo repository.Repository) []*service.Migration {
	var migrationFunc service.MigrationFunc = func(ctx context.Context) error {
		return dbHandler.RunCustomMigrations(
			ctx,
			repo.State().GetAppsAndTeams,
			repo.State().WriteCurrentlyDeployed,
			repo.State().WriteAllReleases,
			repo.State().WriteCurrentEnvironmentLocks,
			repo.State().WriteCurrentApplicationLocks,
			repo.State().WriteCurrentTeamLocks,
			repo.State().GetAllEnvironments,
			repo.State().WriteAllQueuedAppVersions,
			repo.State().WriteAllCommitEvents,
		)
	}

	migrateReleases := func(ctx context.Context) error {
		return dbHandler.RunCustomMigrationReleaseEnvironments(ctx)
	}
	migrateEnvApps := func(ctx context.Context) error {
		return dbHandler.RunCustomMigrationEnvironmentApplications(ctx)
	}
	cleanOutdatedDeployments := func(ctx context.Context) error {
		return dbHandler.RunCustomMigrationCleanOutdatedDeployments(ctx)
	}

	// Migrations here must be IN ORDER, oldest first:
	return []*service.Migration{
		{
			// This first migration is actually a list of migrations that are done in one step:
			Version:   migrations.CreateKuberpultVersion(0, 0, 0),
			Migration: migrationFunc,
		},
		{
			Version:   migrations.CreateKuberpultVersion(0, 0, 1),
			Migration: migrateReleases,
		},
		{
			Version:   migrations.CreateKuberpultVersion(0, 0, 2),
			Migration: migrateEnvApps,
		},
		{
			Version:   migrations.CreateKuberpultVersion(0, 0, 3),
			Migration: cleanOutdatedDeployments,
		},
		// New migrations should be added here:
		// {
		//   Version: ...
		//   Migration: ...
		// }
	}
}

func processEsls(
	ctx context.Context,
	repo repository.Repository,
	dbHandler *db.DBHandler,
	ddMetrics statsd.ClientInterface,
	eslProcessingIdleTimeSeconds int64,
	failOnErrorWithGitPushTags bool,
) error {
	var sleepDuration = backoff.MakeSimpleBackoff(
		time.Second*time.Duration(eslProcessingIdleTimeSeconds),
		time.Second*time.Duration(maxEslProcessingTimeSeconds),
	)
	for {
		span, ctxOneEvent := tracer.StartSpanFromContext(ctx, "processOneEvent")
		wantedSleepTime, err := ProcessOneEvent(ctxOneEvent, repo, dbHandler, ddMetrics, &sleepDuration, failOnErrorWithGitPushTags)
		span.Finish(tracer.WithError(err))
		if err != nil {
			return err
		}
		if wantedSleepTime > 0 {
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
) (
	time.Duration,
	error,
) {
	const transactionRetries = 10
	var transformer repository.Transformer = nil
	var esl *db.EslEventRow = nil
	const readonly = true // we just handle the reading here, there's another transaction for writing the result to the db/git

	logging := logger.FromContext(ctx).Sugar()

	// If KUBERPULT_MINIMIZE_GIT_DATA is enabled, we don't commit on NoOp events, such as lock creation.
	// This means that there is a possibility that two transaction timestamps collide with the same git hash.
	// As such, before executing any transformer, we get the current commit hash so that we can then compare it with the
	// (possibly) new commit hash
	oldCommitId, err := repo.GetHeadCommitId()
	if err != nil {
		d := sleepDuration.NextBackOff()
		if sleepDuration.IsAtMax() {
			return 0, err
		}
		logger.FromContext(ctx).Sugar().Infof("error getting current commid ID, will try again in %v: %v", d, err)
		return d, nil
	}
	err = dbHandler.WithTransactionR(ctx, transactionRetries, readonly, func(ctx context.Context, transaction *sql.Tx) error {
		var err2 error
		transformer, esl, err2 = HandleOneTransformer(ctx, transaction, dbHandler, ddMetrics, repo)
		return err2
	})
	if err != nil {
		if esl == nil {
			logging.Errorf("skipping esl event, because we could not construct esl object: %v", err)
			return 0, err
		}
		logging.Errorf("skipping esl event, because it returned an error: %v", err)
		// after this many tries, we can just skip it:
		err2 := handleFailedEvent(ctx, dbHandler, transactionRetries, esl, err.Error())
		if err2 != nil {
			return 0, fmt.Errorf("error in DBWriteFailedEslEvent %v", err2)
		}
		sleepDuration.Reset()
	} else {
		if transformer == nil {
			sleepDuration.Reset()
			d := sleepDuration.NextBackOff()
			measureGitPushFailures(ddMetrics, logging, false)
			logger.FromContext(ctx).Sugar().Debug("event processing skipped, will try again in %v", d)
			return d, nil
		}
		logging.Infof("event processed successfully, now writing to cutoff and pushing...")
		err = dbHandler.WithTransactionR(ctx, 2, false, func(ctx context.Context, transaction *sql.Tx) error {
			err2 := db.DBWriteCutoff(dbHandler, ctx, transaction, esl.EslVersion)
			if err2 != nil {
				return err2
			}
			err2 = repo.PushRepo(ctx)
			if err2 != nil {
				d := sleepDuration.NextBackOff()
				logger.FromContext(ctx).Sugar().Warnf("error pushing, will try again in %v: %v", d, err2)
				measureGitPushFailures(ddMetrics, logging, true)
				time.Sleep(d)
				return err2
			} else {
				measureGitPushFailures(ddMetrics, logging, false)
			}

			//Get latest commit. Write esl timestamp and commit hash.
			commitId, err := repo.GetHeadCommitId()
			if err != nil {
				return err
			}

			if oldCommitId.String() != commitId.String() { // We only want to write a transaction timestamp if it resulted in a new commit.
				var err3 = dbHandler.DBWriteCommitTransactionTimestamp(ctx, transaction, commitId.String(), esl.Created)
				if err3 != nil {
					return err3
				}
				var gitTag = transformer.GetGitTag()
				if gitTag != "" {
					pushErr := HandleGitTagPush(ctx, repo, gitTag, ddMetrics, failOnErrorWithGitPushTags)
					if pushErr != nil {
						return handleFailedEvent(ctx, dbHandler, transactionRetries, esl,
							fmt.Sprintf("error while pushing the git tag '%s': %v", gitTag, pushErr.Error()))
					}
				}
				return nil
			} else {
				logger.FromContext(ctx).Warn("no commit was created, tagging skipped", zap.String("gitTag", string(transformer.GetGitTag())))
			}
			return nil
		})
		if err != nil {
			//If we fail to push to repo or to update the cutoff, we say that SYNC has failed
			err = dbHandler.WithTransactionR(ctx, 2, false, func(ctx context.Context, transaction *sql.Tx) error {
				return dbHandler.DBBulkUpdateUnsyncedApps(ctx, transaction, db.TransformerID(esl.EslVersion), db.SYNC_FAILED)
			})
			logger.FromContext(ctx).Sugar().Errorf("error updating state for ui: %v", err)
			err3 := repo.FetchAndReset(ctx)
			if err3 != nil {
				d := sleepDuration.NextBackOff()
				logger.FromContext(ctx).Sugar().Warnf("error fetching repo, will try again in %v", d)
				return d, nil
			}
		} else {
			//After a successful transformer processing and pushing to manifest repo, we write that apps are now SYNCED
			err = dbHandler.WithTransactionR(ctx, 2, false, func(ctx context.Context, transaction *sql.Tx) error {
				return dbHandler.DBBulkUpdateUnsyncedApps(ctx, transaction, db.TransformerID(esl.EslVersion), db.SYNCED)
			})
			if err != nil {
				logger.FromContext(ctx).Sugar().Warnf("Failed writing sync status after successful operation! Repo has been updated, but sync status has not. Error: %v", err)
			}
		}
	}
	repo.Notify().Notify() // Notify git sync status

	err = repository.MeasureGitSyncStatus(ctx, ddMetrics, dbHandler)
	if err != nil {
		logger.FromContext(ctx).Sugar().Warnf("Failed sending git sync status metrics: %v", err)
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
			logger.FromContext(ctx).Error("datadog_metrics_error", zap.Error(err), zap.String("metricName", metricName))
		}
	}
}

func measureGitPushFailures(ddMetrics statsd.ClientInterface, log *zap.SugaredLogger, failure bool) {
	if ddMetrics != nil {
		var value float64 = 0
		if failure {
			value = 1
		}
		if err := ddMetrics.Gauge("manifest_export_push_failures", value, []string{}, 1); err != nil {
			log.Error("Error in ddMetrics.Gauge %v", err)
		}
	}
}

func HandleOneTransformer(ctx context.Context, transaction *sql.Tx, dbHandler *db.DBHandler, ddMetrics statsd.ClientInterface, repo repository.Repository) (repository.Transformer, *db.EslEventRow, error) {
	if ddMetrics != nil {
		delaySeconds, delayEvents := dbHandler.GetCurrentDelays(ctx, transaction)
		if err := ddMetrics.Gauge("process_delay_seconds", delaySeconds, []string{}, 1); err != nil {
			log.Error("Error in ddMetrics.Gauge for delay seconds: %v", err)
		}
		if err := ddMetrics.Gauge("process_delay_events", float64(delayEvents), []string{}, 1); err != nil {
			log.Error("Error in ddMetrics.Gauge for delay events: %v", err)
		}
	}
	eslVersion, err := db.DBReadCutoff(dbHandler, ctx, transaction)
	if err != nil {
		return nil, nil, fmt.Errorf("error in DBReadCutoff %v", err)
	}
	if eslVersion == nil {
		logger.FromContext(ctx).Sugar().Infof("did not find cutoff")
	}
	esl, err := dbHandler.DBReadEslEvent(ctx, transaction, eslVersion)
	if err != nil {
		return nil, esl, fmt.Errorf("error in readEslEvent %v", err)
	}
	if esl == nil {
		// no event found
		return nil, nil, nil
	}
	transformer, err := processEslEvent(ctx, repo, esl, transaction)
	return transformer, esl, err
}

func processEslEvent(ctx context.Context, repo repository.Repository, esl *db.EslEventRow, tx *sql.Tx) (repository.Transformer, error) {
	if esl == nil {
		return nil, fmt.Errorf("esl event nil")
	}
	var t repository.Transformer
	t, err := getTransformer(ctx, esl.EventType)
	if err != nil {
		return nil, fmt.Errorf("get transformer error %v", err)
	}
	if t == nil {
		// no error, but also no transformer to process:
		return nil, nil
	}
	logger.FromContext(ctx).Sugar().Infof("processEslEvent: unmarshal \n%s", esl.EventJson)
	err = json.Unmarshal(([]byte)(esl.EventJson), &t)
	if err != nil {
		return nil, err
	}
	t.SetEslVersion(db.TransformerID(esl.EslVersion))
	logger.FromContext(ctx).Sugar().Infof("read esl event of type (%s) event=%v", t.GetDBEventType(), t)
	t.SetCreationTimestamp(esl.Created)
	err = repo.Apply(ctx, tx, t)
	if err != nil {
		return nil, fmt.Errorf("error while running repo apply: %v", err)
	}

	logger.FromContext(ctx).Sugar().Infof("Applied transformer succesfully event=%s", t.GetDBEventType())
	return t, nil
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
	}
	return nil, fmt.Errorf("could not find transformer for event type %v", eslEventType)
}

func checkReleaseVersionLimit(limit uint) error {
	if limit < minReleaseVersionsLimit || limit > maxReleaseVersionsLimit {
		return releaseVersionsLimitError{limit: limit}
	}
	return nil
}

func shouldRunCustomMigrations(checkCustomMigrations, minimizeGitData bool) bool {
	return checkCustomMigrations && !minimizeGitData //If `minimizeGitData` is enabled we can't make sure we have all the information on the repository to perform all the migrations
}
