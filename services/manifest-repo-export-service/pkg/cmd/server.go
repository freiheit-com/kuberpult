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
	"github.com/cenkalti/backoff/v4"
	cutoff "github.com/freiheit-com/kuberpult/services/manifest-repo-export-service/pkg/db"
	"strconv"
	"time"

	"encoding/json"
	"fmt"
	"os"

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
)

func RunServer() {
	_ = logger.Wrap(context.Background(), func(ctx context.Context) error {
		err := Run(ctx)
		if err != nil {
			logger.FromContext(ctx).Sugar().Errorf("error in startup: %v %#v", err, err)
		}
		return nil
	})
}

func Run(ctx context.Context) error {
	log := logger.FromContext(ctx).Sugar()

	logger.FromContext(ctx).Info("Startup")

	dbLocation, err := readEnvVar("KUBERPULT_DB_LOCATION")
	if err != nil {
		return err
	}
	dbName, err := readEnvVar("KUBERPULT_DB_NAME")
	if err != nil {
		return err
	}
	dbOption, err := readEnvVar("KUBERPULT_DB_OPTION")
	if err != nil {
		return err
	}
	dbUserName, err := readEnvVar("KUBERPULT_DB_USER_NAME")
	if err != nil {
		return err
	}
	dbPassword, err := readEnvVar("KUBERPULT_DB_USER_PASSWORD")
	if err != nil {
		return err
	}
	dbAuthProxyPort, err := readEnvVar("KUBERPULT_DB_AUTH_PROXY_PORT")
	if err != nil {
		return err
	}
	gitUrl, err := readEnvVar("KUBERPULT_GIT_URL")
	if err != nil {
		return err
	}
	gitBranch, err := readEnvVar("KUBERPULT_GIT_BRANCH")
	if err != nil {
		return err
	}
	gitSshKey, err := readEnvVar("KUBERPULT_GIT_SSH_KEY")
	if err != nil {
		return err
	}
	gitSshKnownHosts, err := readEnvVar("KUBERPULT_GIT_SSH_KNOWN_HOSTS")
	if err != nil {
		return err
	}

	enableMetricsString, err := readEnvVar("KUBERPULT_ENABLE_METRICS")
	if err != nil {
		log.Info("datadog metrics are disabled")
	}
	enableMetrics := enableMetricsString == "true"
	dataDogStatsAddr := "127.0.0.1:8125"
	if enableMetrics {
		dataDogStatsAddrEnv, err := readEnvVar("KUBERPULT_DOGSTATSD_ADDR")
		if err != nil {
			log.Infof("using default dogStatsAddr: %s", dataDogStatsAddr)
		} else {
			dataDogStatsAddr = dataDogStatsAddrEnv
		}
	}

	enableTracesString, err := readEnvVar("KUBERPULT_ENABLE_TRACING")
	if err != nil {
		log.Info("datadog traces are disabled")
	}
	enableTraces := enableTracesString == "true"
	if enableTraces {
		tracer.Start()
		defer tracer.Stop()
	}

	releaseVersionLimitStr, err := readEnvVar("KUBERPULT_RELEASE_VERSIONS_LIMIT")
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

	var eslProcessingIdleTimeSeconds uint64
	if val, exists := os.LookupEnv("KUBERPULT_ESL_PROCESSING_BACKOFF"); !exists {
		log.Infof("environment variable KUBERPULT_ESL_PROCESSING_BACKOFF is not set, using default backoff of 10 seconds")
		eslProcessingIdleTimeSeconds = 10
	} else {
		eslProcessingIdleTimeSeconds, err = strconv.ParseUint(val, 10, 64)
		if err != nil {
			return fmt.Errorf("error converting KUBERPULT_ESL_PROCESSING_BACKOFF, error: %w", err)
		}
	}

	networkTimeoutSecondsStr, err := readEnvVar("KUBERPULT_NETWORK_TIMEOUT_SECONDS")
	if err != nil {
		return err
	}
	networkTimeoutSeconds, err := strconv.ParseUint(networkTimeoutSecondsStr, 10, 64)
	if err != nil {
		return fmt.Errorf("error parsing KUBERPULT_NETWORK_TIMEOUT_SECONDS, error: %w", err)
	}

	argoCdGenerateFilesString, err := readEnvVar("KUBERPULT_ARGO_CD_GENERATE_FILES")
	if err != nil {
		return err
	}
	argoCdGenerateFiles := argoCdGenerateFilesString == "true"

	var dbCfg db.DBConfig
	if dbOption == "postgreSQL" {
		dbCfg = db.DBConfig{
			DbHost:         dbLocation,
			DbPort:         dbAuthProxyPort,
			DriverName:     "postgres",
			DbName:         dbName,
			DbPassword:     dbPassword,
			DbUser:         dbUserName,
			MigrationsPath: "",
			WriteEslOnly:   false,
		}
	} else if dbOption == "sqlite" {
		dbCfg = db.DBConfig{
			DbHost:         dbLocation,
			DbPort:         dbAuthProxyPort,
			DriverName:     "sqlite3",
			DbName:         dbName,
			DbPassword:     dbPassword,
			DbUser:         dbUserName,
			MigrationsPath: "",
			WriteEslOnly:   false,
		}
	} else {
		logger.FromContext(ctx).Fatal("Cannot start without DB configuration was provided.")
	}
	dbHandler, err := db.Connect(dbCfg)
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
		CommitterEmail: "noemail@example.com", // TODO will be handled in Ref SRX-PA568W
		CommitterName:  "noname",
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
		DBHandler:           dbHandler,
	}

	repo, err := repository.New(ctx, cfg)
	if err != nil {
		return fmt.Errorf("repository.new failed %v", err)
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
				reflection.Register(srv)
			},
		},
		Background: []setup.BackgroundTaskConfig{
			{
				Shutdown: nil,
				Name:     "processEsls",
				Run: func(ctx context.Context, reporter *setup.HealthReporter) error {
					reporter.ReportReady("Processing Esls")
					return processEsls(ctx, repo, dbHandler, ddMetrics, eslProcessingIdleTimeSeconds)
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

func processEsls(ctx context.Context, repo repository.Repository, dbHandler *db.DBHandler, ddMetrics statsd.ClientInterface, eslProcessingIdleTimeSeconds uint64) error {
	log := logger.FromContext(ctx).Sugar()
	sleepDuration := createBackOffProvider(time.Second * time.Duration(eslProcessingIdleTimeSeconds))

	const transactionRetries = 10
	for {
		var transformer repository.Transformer = nil
		var esl *db.EslEventRow = nil
		const readonly = true // we just handle the reading here, there's another transaction for writing the result to the db/git
		err := dbHandler.WithTransactionR(ctx, transactionRetries, readonly, func(ctx context.Context, transaction *sql.Tx) error {
			var err2 error = nil
			transformer, esl, err2 = handleOneEvent(ctx, transaction, dbHandler, ddMetrics, repo)
			return err2
		})
		if err != nil {
			if esl == nil {
				log.Errorf("skipping esl event, because we could not construct esl object: %v", err)
				return err
			}
			log.Errorf("skipping esl event, because it returned an error: %v", err)
			// after this many tries, we can just skip it:
			err2 := dbHandler.WithTransactionR(ctx, transactionRetries, false, func(ctx context.Context, transaction *sql.Tx) error {
				err3 := dbHandler.DBWriteFailedEslEvent(ctx, transaction, esl)
				if err3 != nil {
					return err3
				}
				return cutoff.DBWriteCutoff(dbHandler, ctx, transaction, esl.EslVersion)
			})
			if err2 != nil {
				return fmt.Errorf("error in DBWriteFailedEslEvent %v", err2)
			}
			sleepDuration.Reset()
		} else {
			if transformer == nil {
				sleepDuration.Reset()
				d := sleepDuration.NextBackOff()
				logger.FromContext(ctx).Sugar().Warnf("event processing skipped, will try again in %v", d)
				time.Sleep(d)
				continue
			}
			log.Infof("event processed successfully, now writing to cutoff and pushing...")
			err = dbHandler.WithTransactionR(ctx, 2, false, func(ctx context.Context, transaction *sql.Tx) error {
				err2 := cutoff.DBWriteCutoff(dbHandler, ctx, transaction, esl.EslVersion)
				if err2 != nil {
					return err2
				}
				err2 = repo.PushRepo(ctx)
				if err2 != nil {
					d := sleepDuration.NextBackOff()
					logger.FromContext(ctx).Sugar().Warnf("error pushing, will try again in %v", d)
					if ddMetrics != nil {
						if err := ddMetrics.Gauge("manifest_export_push_failures", 1, []string{}, 1); err != nil {
							log.Error("Error in ddMetrics.Gauge %v", err)
						}
					}
					time.Sleep(d)
				}
				return err2
			})
			if err != nil {
				err3 := repo.FetchAndReset(ctx)
				if err3 != nil {
					d := sleepDuration.NextBackOff()
					logger.FromContext(ctx).Sugar().Warnf("error fetching repo, will try again in %v", d)
					time.Sleep(d)
				}
			}
		}
	}
}

func handleOneEvent(ctx context.Context, transaction *sql.Tx, dbHandler *db.DBHandler, ddMetrics statsd.ClientInterface, repo repository.Repository) (repository.Transformer, *db.EslEventRow, error) {
	log := logger.FromContext(ctx).Sugar()
	eslVersion, err := cutoff.DBReadCutoff(dbHandler, ctx, transaction)
	if err != nil {
		return nil, nil, fmt.Errorf("error in DBReadCutoff %v", err)
	}
	if eslVersion == nil {
		log.Infof("did not find cutoff")
	} else {
		log.Infof("found cutoff: %d", *eslVersion)
	}
	esl, err := readEslEvent(ctx, transaction, eslVersion, log, dbHandler)
	if err != nil {
		return nil, esl, fmt.Errorf("error in readEslEvent %v", err)
	}
	if ddMetrics != nil {
		now := time.Now().UTC()
		processDelay, err := calculateProcessDelay(ctx, esl, now)
		if err != nil {
			log.Error("Error in calculateProcessDelay %v", err)
		}
		if processDelay < 0 {
			log.Warn("process delay is negative: esl-time: %v, now: %v, delay: %v", esl.Created, now, processDelay)
		}
		if err := ddMetrics.Gauge("process_delay_seconds", processDelay, []string{}, 1); err != nil {
			log.Error("Error in ddMetrics.Gauge %v", err)
		}
	}
	if esl == nil {
		// no event found
		return nil, nil, nil
	}
	transformer, err := processEslEvent(ctx, repo, esl, transaction)
	return transformer, esl, err

}

func createBackOffProvider(initialInterval time.Duration) backoff.BackOff {
	return backoff.NewExponentialBackOff(
		backoff.WithMultiplier(2),
		backoff.WithRandomizationFactor(0.5),
		backoff.WithInitialInterval(initialInterval),
		backoff.WithMaxElapsedTime(10*time.Minute),
	)
}

func calculateProcessDelay(_ context.Context, nextEslToProcess *db.EslEventRow, currentTime time.Time) (float64, error) {
	if nextEslToProcess == nil {
		return 0, nil
	}
	if nextEslToProcess.Created.IsZero() {
		return 0, nil
	}
	diff := currentTime.Sub(nextEslToProcess.Created).Seconds()
	return diff, nil
}

func readEslEvent(ctx context.Context, transaction *sql.Tx, eslVersion *db.EslVersion, log *zap.SugaredLogger, dbHandler *db.DBHandler) (*db.EslEventRow, error) {
	if eslVersion == nil {
		log.Warnf("no cutoff found, starting at the beginning of time.")
		// no read cutoff yet, we have to start from the beginning
		esl, err := dbHandler.DBReadEslEventInternal(ctx, transaction, true)
		if err != nil {
			return nil, err
		}
		if esl == nil {
			log.Warnf("no esl events found")
			return nil, nil
		}
		return esl, nil
	} else {
		log.Warnf("cutoff found, starting at t>cutoff: %d", *eslVersion)
		esl, err := dbHandler.DBReadEslEventLaterThan(ctx, transaction, *eslVersion)
		if err != nil {
			return nil, err
		}
		return esl, nil
	}
}

var debugValue int64 = 0

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
	logger.FromContext(ctx).Sugar().Infof("processEslEvent: unmarshal \n%s\n", esl.EventJson)
	err = json.Unmarshal(([]byte)(esl.EventJson), &t)
	if err != nil {
		return nil, err
	}
	t.SetEslVersion(db.TransformerID(esl.EslVersion))
	logger.FromContext(ctx).Sugar().Infof("read esl event of type (%s) event=%v", t.GetDBEventType(), t)

	err = repo.Apply(ctx, tx, t)
	if err != nil {
		return nil, fmt.Errorf("error while running repo apply: %v", err)
	}

	debugValue++
	if debugValue%3 == 0 {
		return nil, fmt.Errorf("random error")
	}
	logger.FromContext(ctx).Sugar().Infof("Applied transformer succesfully event=%s", t.GetDBEventType())
	return t, nil
}

// getTransformer returns an empty transformer of the type according to esl.EventType
func getTransformer(ctx context.Context, eslEventType db.EventType) (repository.Transformer, error) {
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
		logger.FromContext(ctx).Sugar().Warn("Release train event found. No action will be taken and event will be skipped.")
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
	case db.EvtUndeployApplication:
		//exhaustruct:ignore
		return &repository.UndeployApplication{}, nil
	}
	return nil, fmt.Errorf("could not find transformer for event type %v", eslEventType)
}

func readEnvVar(envName string) (string, error) {
	envValue, ok := os.LookupEnv(envName)
	if !ok {
		return "", fmt.Errorf("could not read environment variable '%s'", envName)
	}
	return envValue, nil
}

func checkReleaseVersionLimit(limit uint) error {
	if limit < minReleaseVersionsLimit || limit > maxReleaseVersionsLimit {
		return releaseVersionsLimitError{limit: limit}
	}
	return nil
}
