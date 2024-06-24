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
	"strconv"
	"time"

	"encoding/json"
	"fmt"
	"os"

	"github.com/DataDog/datadog-go/v5/statsd"
	"github.com/freiheit-com/kuberpult/pkg/db"
	"github.com/freiheit-com/kuberpult/pkg/logger"
	cutoff "github.com/freiheit-com/kuberpult/services/manifest-repo-export-service/pkg/db"
	"github.com/freiheit-com/kuberpult/services/manifest-repo-export-service/pkg/repository"
	"go.uber.org/zap"
)

func storageBackend(enableSqlite bool) repository.StorageBackend {
	if enableSqlite {
		return repository.SqliteBackend
	} else {
		return repository.GitBackend
	}
}

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
	// not that this is for the git storage backand, not our database:
	enableSqliteStorageBackendString, err := readEnvVar("KUBERPULT_ENABLE_SQLITE")
	if err != nil {
		return err
	}
	enableMetricsString, err := readEnvVar("KUBERPULT_ENABLE_METRICS")
	if err != nil {
		return err
	}
	enableMetrics := enableMetricsString == "true"
	DatatDogStatsAddr, err := readEnvVar("KUBERPULT_DOGSTATSD_ADDR")
	if err != nil {
		return err
	}

	releaseVersionLimit := 20
	//if val, exists := os.LookupEnv("KUBERPULT_RELEASE_VERSIONS_LIMIT"); !exists {
	//	log.Infof("environment variable KUBERPULT_RELEASE_VERSIONS_LIMIT is not set, using default releaseVersionLimit of 20.")
	//	releaseVersionLimit = 20
	//} else {
	//	releaseVersionLimit, err = strconv.ParseUint(val, 10, 64)
	//	if err != nil {
	//		return fmt.Errorf("error converting KUBERPULT_RELEASE_VERSIONS_LIMIT, error: %w", err)
	//	}
	//
	if err := checkReleaseVersionLimit(uint(releaseVersionLimit)); err != nil {
		return fmt.Errorf("error parsing KUBERPULT_RELEASE_VERSIONS_LIMIT, error: %w", err)
	}
	
	var eslProcessingBackoff uint64
	if val, exists := os.LookupEnv("KUBERPULT_ESL_PROCESSING_BACKOFF"); !exists {
		log.Infof("environment variable KUBERPULT_ESL_PROCESSING_BACKOFF is not set, using default backoff of 10 seconds")
		eslProcessingBackoff = 10
	} else {
		eslProcessingBackoff, err = strconv.ParseUint(val, 10, 64)
		if err != nil {
			return fmt.Errorf("error converting KUBERPULT_ESL_PROCESSING_BACKOFF, error: %w", err)
		}
	}

	enableSqliteStorageBackend := enableSqliteStorageBackendString == "true"

	argoCdGenerateFilesString, err := readEnvVar("KUBERPULT_ARGO_CD_GENERATE_FILES")
	if err != nil {
		return err
	}
	argoCdGenerateFiles := argoCdGenerateFilesString == "true"

	var dbCfg db.DBConfig
	if dbOption == "cloudsql" {
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
		ddMetrics, err = statsd.New(DatatDogStatsAddr, statsd.WithNamespace("Kuberpult"))
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
		Branch:                 gitBranch,
		NetworkTimeout:         120 * time.Second,
		GcFrequency:            20,
		BootstrapMode:          false,
		EnvironmentConfigsPath: "./environment_configs.json",
		StorageBackend:         storageBackend(enableSqliteStorageBackend),
		ArgoCdGenerateFiles:    argoCdGenerateFiles,
		ReleaseVersionsLimit:   uint(releaseVersionLimit),
		DBHandler:              dbHandler,
	}

	repo, err := repository.New(ctx, cfg)
	fmt.Println(repo.State().ReleaseVersionsLimit)
	if err != nil {
		return fmt.Errorf("repository.new failed %v", err)
	}

	for {
		eslTableEmpty := false
		eslEventSkipped := false

		err = dbHandler.WithTransaction(ctx, func(ctx context.Context, transaction *sql.Tx) error {
			eslId, err := cutoff.DBReadCutoff(dbHandler, ctx, transaction)
			if err != nil {
				return fmt.Errorf("error in DBReadCutoff %v", err)
			}
			if eslId == nil {
				log.Infof("did not find cutoff")
			} else {
				log.Infof("found cutoff: %d", *eslId)
			}
			esl, err := readEslEvent(ctx, transaction, eslId, log, dbHandler)
			if err != nil {
				return fmt.Errorf("error in readEslEvent %v", err)
			}
			if ddMetrics != nil {
				processDelay, err := calculateProcessDelay(ctx, esl, time.Now())
				if err != nil {
					log.Error("Error in calculateProcessDelay %v", err)
				}
				if err := ddMetrics.Gauge("process_delay_seconds", processDelay, []string{}, 1); err != nil {
					log.Error("Error in ddMetrics.Gauge %v", err)
				}
			}
			if esl == nil {
				log.Warn("event processing skipped: no esl event found")
				eslTableEmpty = true
				return nil
			}
			transformer, err := processEslEvent(ctx, repo, esl, transaction)
			if err != nil {
				return fmt.Errorf("error in processEslEvent %v", err)
			}
			if transformer == nil {
				log.Warn("event processing skipped")
				eslEventSkipped = true
				return nil
			}
			log.Infof("event processed successfully, now writing to cutoff...")
			err = cutoff.DBWriteCutoff(dbHandler, ctx, transaction, esl.EslId)
			if err != nil {
				return fmt.Errorf("error in DBWriteCutoff %v", err)
			}
			return nil
		})
		if err != nil {
			return fmt.Errorf("error in transaction %v", err)
		}
		if eslEventSkipped || eslTableEmpty {
			d := time.Second * time.Duration(eslProcessingBackoff)
			log.Infof("sleeping for %v before processing the next event", d)
			time.Sleep(d)
		}
	}
}

func calculateProcessDelay(ctx context.Context, nextEslToProcess *db.EslEventRow, currentTime time.Time) (float64, error) {
	if nextEslToProcess == nil {
		return 0, nil
	}
	if nextEslToProcess.Created.IsZero() {
		return 0, nil
	}
	diff := currentTime.Sub(nextEslToProcess.Created).Seconds()
	return diff, nil
}

func readEslEvent(ctx context.Context, transaction *sql.Tx, eslId *db.EslId, log *zap.SugaredLogger, dbHandler *db.DBHandler) (*db.EslEventRow, error) {
	if eslId == nil {
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
		//log.Warnf("found esl event %v of type %s", esl, esl.EventType)
	} else {
		log.Warnf("cutoff found, starting at t>cutoff: %d", *eslId)
		esl, err := dbHandler.DBReadEslEventLaterThan(ctx, transaction, *eslId)
		if err != nil {
			return nil, err
		}
		return esl, nil
	}
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
	logger.FromContext(ctx).Sugar().Infof("processEslEvent: unmarshal \n%s\n", esl.EventJson)
	err = json.Unmarshal(([]byte)(esl.EventJson), &t)
	if err != nil {
		return nil, err
	}
	logger.FromContext(ctx).Sugar().Infof("read esl event of type (%s) event=%v", t.GetDBEventType(), t)

	err = repo.Apply(ctx, tx, t)
	if err != nil {
		return nil, fmt.Errorf("error while running repo apply: %v", err)
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
	default:
		logger.FromContext(ctx).Sugar().Warnf("Found an unknown event %s. No further events will be processed.", eslEventType)
		return nil, nil
	}
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
