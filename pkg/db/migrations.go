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

package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"github.com/freiheit-com/kuberpult/pkg/logger"
	"github.com/freiheit-com/kuberpult/pkg/sorting"
	"github.com/golang-migrate/migrate/v4"
	"gopkg.in/DataDog/dd-trace-go.v1/ddtrace/tracer"
	"reflect"
	"slices"
	"time"
)

/**
This package takes care of 2 migrations
1) SQL migrations: these are typically "create table" statements
2) Custom migrations: go functions that run to initially fill the new tables from step 1)

*/

func RunDBMigrations(cfg DBConfig) error {
	d, err := Connect(cfg)
	if err != nil {
		return fmt.Errorf("DB Error opening DB connection. Error:  %w\n", err)
	}
	defer d.DB.Close()

	m, err := d.getMigrationHandler()
	if err != nil {
		return fmt.Errorf("Error creating migration instance. Error: %w\n", err)
	}
	defer m.Close()
	if err := m.Up(); err != nil {
		if !errors.Is(err, migrate.ErrNoChange) {
			return fmt.Errorf("Error running DB migrations. Error: %w\n", err)
		}
	}
	return nil
}

func (h *DBHandler) getMigrationHandler() (*migrate.Migrate, error) {
	if h.DriverName == "postgres" {
		return migrate.NewWithDatabaseInstance("file://"+h.MigrationsPath, h.DbName, *h.DBDriver)
	} else if h.DriverName == "sqlite3" {
		return migrate.NewWithDatabaseInstance("file://"+h.MigrationsPath, "", *h.DBDriver) //FIX ME
	}
	return nil, fmt.Errorf("Driver: '%s' not supported. Supported: postgres and sqlite3.", h.DriverName)
}

type AllDeployments []Deployment
type AllEnvLocks map[string][]EnvironmentLock
type AllReleases map[uint64]ReleaseWithManifest // keys: releaseVersion; value: release with manifests

// GetAllDeploymentsFun and other functions here are used during migration.
// They are supposed to read data from files in the manifest repo,
// and therefore should not need to access the Database at all.
type GetAllDeploymentsFun = func(ctx context.Context, transaction *sql.Tx) (AllDeployments, error)
type GetAllAppLocksFun = func(ctx context.Context) (AllAppLocks, error)

type AllAppLocks map[string]map[string][]ApplicationLock // EnvName-> AppName -> []Locks
type AllTeamLocks map[string]map[string][]TeamLock       // EnvName-> Team -> []Locks
type AllQueuedVersions map[string]map[string]*int64      // EnvName-> AppName -> queuedVersion

type GetAllEnvLocksFun = func(ctx context.Context) (AllEnvLocks, error)
type GetAllTeamLocksFun = func(ctx context.Context) (AllTeamLocks, error)
type GetAllReleasesFun = func(ctx context.Context, app string) (AllReleases, error)
type GetAllQueuedVersionsFun = func(ctx context.Context) (AllQueuedVersions, error)

// GetAllAppsFun returns a map where the Key is an app name, and the value is a team name of that app
type GetAllAppsFun = func() (map[string]string, error)

func (h *DBHandler) RunCustomMigrations(
	ctx context.Context,
	getAllAppsFun GetAllAppsFun,
	getAllDeploymentsFun GetAllDeploymentsFun,
	getAllReleasesFun GetAllReleasesFun,
	getAllEnvLocksFun GetAllEnvLocksFun,
	getAllAppLocksFun GetAllAppLocksFun,
	getAllTeamLocksFun GetAllTeamLocksFun,
	getAllQueuedVersionsFun GetAllQueuedVersionsFun,
) error {
	span, ctx := tracer.StartSpanFromContext(ctx, "RunCustomMigrations")
	defer span.Finish()
	err := h.RunCustomMigrationAllAppsTable(ctx, getAllAppsFun)
	if err != nil {
		return err
	}
	err = h.RunCustomMigrationApps(ctx, getAllAppsFun)
	if err != nil {
		return err
	}
	err = h.RunCustomMigrationDeployments(ctx, getAllDeploymentsFun)
	if err != nil {
		return err
	}
	err = h.RunCustomMigrationReleases(ctx, getAllAppsFun, getAllReleasesFun)
	if err != nil {
		return err
	}
	err = h.RunCustomMigrationEnvLocks(ctx, getAllEnvLocksFun)
	if err != nil {
		return err
	}
	err = h.RunCustomMigrationAppLocks(ctx, getAllAppLocksFun)
	if err != nil {
		return err
	}
	err = h.RunCustomMigrationTeamLocks(ctx, getAllTeamLocksFun)
	if err != nil {
		return err
	}
	err = h.RunCustomMigrationQueuedApplicationVersions(ctx, getAllQueuedVersionsFun)
	if err != nil {
		return err
	}
	return nil
}

func (h *DBHandler) RunCustomMigrationAllAppsTable(ctx context.Context, getAllAppsFun GetAllAppsFun) error {
	return h.WithTransaction(ctx, true, func(ctx context.Context, transaction *sql.Tx) error {
		l := logger.FromContext(ctx).Sugar()
		allAppsDb, err := h.DBSelectAllApplications(ctx, transaction)
		if err != nil {
			l.Warnf("could not get applications from database - assuming the manifest repo is correct: %v", err)
			allAppsDb = nil
		}

		allAppsRepo, err := getAllAppsFun()
		if err != nil {
			return fmt.Errorf("could not get applications to run custom migrations: %v", err)
		}
		var version int64
		if allAppsDb != nil {
			slices.Sort(allAppsDb.Apps)
			version = allAppsDb.Version
		} else {
			version = 1
		}
		sortedApps := sorting.SortKeys(allAppsRepo)

		if allAppsDb != nil && reflect.DeepEqual(allAppsDb.Apps, sortedApps) {
			// nothing to do
			logger.FromContext(ctx).Sugar().Infof("Nothing to do, all apps are equal")
			return nil
		}
		// if there is any difference, we assume the manifest wins over the database state,
		// so we use `allAppsRepo`:
		return h.DBWriteAllApplications(ctx, transaction, version, sortedApps)
	})
}

func (h *DBHandler) RunCustomMigrationApps(ctx context.Context, getAllAppsFun GetAllAppsFun) error {
	return h.WithTransaction(ctx, false, func(ctx context.Context, transaction *sql.Tx) error {
		dbApp, err := h.DBSelectAnyApp(ctx, transaction)
		if err != nil {
			return fmt.Errorf("could not get dbApp from database - assuming the manifest repo is correct: %v", err)
		}
		if dbApp != nil {
			// the migration was already done
			logger.FromContext(ctx).Info("migration to apps was done already")
			return nil
		}

		appsMap, err := getAllAppsFun()
		if err != nil {
			return fmt.Errorf("could not get dbApp to run custom migrations: %v", err)
		}

		for app := range appsMap {
			team := appsMap[app]
			err = h.DBInsertApplication(ctx, transaction, app, InitialEslId, AppStateChangeMigrate, DBAppMetaData{Team: team})
			if err != nil {
				return fmt.Errorf("could not write dbApp %s: %v", app, err)
			}
		}
		return nil
	})
}

func (h *DBHandler) RunCustomMigrationDeployments(ctx context.Context, getAllDeploymentsFun GetAllDeploymentsFun) error {
	return h.WithTransaction(ctx, false, func(ctx context.Context, transaction *sql.Tx) error {
		l := logger.FromContext(ctx).Sugar()
		allAppsDb, err := h.DBSelectAnyDeployment(ctx, transaction)
		if err != nil {
			l.Warnf("could not get applications from database - assuming the manifest repo is correct: %v", err)
			allAppsDb = nil
		}
		if allAppsDb != nil {
			l.Warnf("There are already deployments in the DB - skipping migrations")
			return nil
		}

		allDeploymentsInRepo, err := getAllDeploymentsFun(ctx, transaction)
		if err != nil {
			return fmt.Errorf("could not get current deployments to run custom migrations: %v", err)
		}

		for i := range allDeploymentsInRepo {
			deploymentInRepo := allDeploymentsInRepo[i]
			err = h.DBWriteDeployment(ctx, transaction, deploymentInRepo, 0)
			if err != nil {
				return fmt.Errorf("error writing Deployment to DB for app %s in env %s: %v",
					deploymentInRepo.App, deploymentInRepo.Env, err)
			}
		}
		return nil
	})
}

func (h *DBHandler) RunCustomMigrationReleases(ctx context.Context, getAllAppsFun GetAllAppsFun, getAllReleasesFun GetAllReleasesFun) error {
	return h.WithTransaction(ctx, false, func(ctx context.Context, transaction *sql.Tx) error {
		l := logger.FromContext(ctx).Sugar()
		allReleasesDb, err := h.DBSelectAnyRelease(ctx, transaction)
		if err != nil {
			l.Warnf("could not get releases from database - assuming the manifest repo is correct: %v", err)
		}
		if allReleasesDb != nil {
			l.Warnf("There are already deployments in the DB - skipping migrations")
			return nil
		}

		allAppsMap, err := getAllAppsFun()
		if err != nil {
			return err
		}
		for app := range allAppsMap {
			l.Infof("processing app %s ...", app)

			releases, err := getAllReleasesFun(ctx, app)
			if err != nil {
				return fmt.Errorf("geAllReleases failed %v", err)
			}

			releaseNumbers := []int64{}
			for r := range releases {
				repoRelease := releases[r]
				dbRelease := DBReleaseWithMetaData{
					EslId:         InitialEslId,
					Created:       time.Now().UTC(),
					ReleaseNumber: repoRelease.Version,
					App:           app,
					Manifests: DBReleaseManifests{
						Manifests: repoRelease.Manifests,
					},
					Metadata: DBReleaseMetaData{
						SourceAuthor:   repoRelease.SourceAuthor,
						SourceCommitId: repoRelease.SourceCommitId,
						SourceMessage:  repoRelease.SourceMessage,
						DisplayVersion: repoRelease.DisplayVersion,
					},
					Deleted: false,
				}
				err = h.DBInsertRelease(ctx, transaction, dbRelease, InitialEslId-1)
				if err != nil {
					return fmt.Errorf("error writing Release to DB for app %s: %v", app, err)
				}
				releaseNumbers = append(releaseNumbers, int64(repoRelease.Version))
			}
			l.Infof("done with app %s", app)
			err = h.DBInsertAllReleases(ctx, transaction, app, releaseNumbers, InitialEslId-1)
			if err != nil {
				return fmt.Errorf("error writing all_releases to DB for app %s: %v", app, err)
			}
		}
		return nil
	})
}

func (h *DBHandler) RunCustomMigrationEnvLocks(ctx context.Context, getAllEnvLocksFun GetAllEnvLocksFun) error {
	return h.WithTransaction(ctx, false, func(ctx context.Context, transaction *sql.Tx) error {
		l := logger.FromContext(ctx).Sugar()
		allEnvLocksDb, err := h.DBSelectAnyActiveEnvLocks(ctx, transaction)
		if err != nil {
			l.Infof("could not get environment locks from database - assuming the manifest repo is correct: %v", err)
			allEnvLocksDb = nil
		}
		if allEnvLocksDb != nil {
			l.Infof("There are already environment locks in the DB - skipping migrations")
			return nil
		}

		allEnvLocksInRepo, err := getAllEnvLocksFun(ctx)
		if err != nil {
			return fmt.Errorf("could not get current environment locks to run custom migrations: %v", err)
		}

		for envName, locks := range allEnvLocksInRepo {
			var activeLockIds []string
			for _, currentLock := range locks {
				activeLockIds = append(activeLockIds, currentLock.LockID)

				err = h.DBWriteEnvironmentLockInternal(ctx, transaction, currentLock, 0, true)
				if err != nil {
					return fmt.Errorf("error writing environment locks to DB for environment %s: %v",
						envName, err)
				}
			}

			if len(activeLockIds) == 0 {
				activeLockIds = []string{}
			}
			err = h.DBWriteAllEnvironmentLocks(ctx, transaction, 0, envName, activeLockIds)
			if err != nil {
				return fmt.Errorf("error writing environment locks ids to DB for environment %s: %v",
					envName, err)
			}
		}

		return nil
	})
}

func (h *DBHandler) RunCustomMigrationAppLocks(ctx context.Context, getAllAppLocksFun GetAllAppLocksFun) error {
	return h.WithTransaction(ctx, false, func(ctx context.Context, transaction *sql.Tx) error {
		l := logger.FromContext(ctx).Sugar()
		allAppLocksDb, err := h.DBSelectAnyActiveAppLock(ctx, transaction)
		if err != nil {
			l.Infof("could not get application locks from database - assuming the manifest repo is correct: %v", err)
			allAppLocksDb = nil
		}
		if allAppLocksDb != nil {
			l.Infof("There are already application locks in the DB - skipping migrations")
			return nil
		}

		allAppLocksInRepo, err := getAllAppLocksFun(ctx)
		if err != nil {
			return fmt.Errorf("could not get current application locks to run custom migrations: %v", err)
		}

		for envName, apps := range allAppLocksInRepo {
			for appName, currentAppLocks := range apps {
				var activeLockIds []string
				for _, currentLock := range currentAppLocks {
					activeLockIds = append(activeLockIds, currentLock.LockID)
					err = h.DBWriteApplicationLockInternal(ctx, transaction, currentLock, 0, true)
					if err != nil {
						return fmt.Errorf("error writing application locks to DB for application '%s' on '%s': %v",
							appName, envName, err)
					}
				}
				if len(activeLockIds) == 0 {
					activeLockIds = []string{}
				}

				err := h.DBWriteAllAppLocks(ctx, transaction, 0, envName, appName, activeLockIds)
				if err != nil {
					return fmt.Errorf("error writing existing locks to DB for application '%s' on environment '%s': %v",
						appName, envName, err)
				}
			}
		}
		return nil
	})
}

func (h *DBHandler) RunCustomMigrationTeamLocks(ctx context.Context, getAllTeamLocksFun GetAllTeamLocksFun) error {
	return h.WithTransaction(ctx, false, func(ctx context.Context, transaction *sql.Tx) error {
		l := logger.FromContext(ctx).Sugar()
		allTeamLocksDb, err := h.DBSelectAnyActiveTeamLock(ctx, transaction)
		if err != nil {
			l.Infof("could not get team locks from database - assuming the manifest repo is correct: %v", err)
			allTeamLocksDb = nil
		}
		if allTeamLocksDb != nil {
			l.Infof("There are already team locks in the DB - skipping migrations")
			return nil
		}

		allTeamLocksInRepo, err := getAllTeamLocksFun(ctx)
		if err != nil {
			return fmt.Errorf("could not get current team locks to run custom migrations: %v", err)
		}

		for envName, apps := range allTeamLocksInRepo {
			for teamName, currentTeamLocks := range apps {
				var activeLockIds []string
				for _, currentLock := range currentTeamLocks {
					activeLockIds = append(activeLockIds, currentLock.LockID)
					err = h.DBWriteTeamLockInternal(ctx, transaction, currentLock, 0, true)
					if err != nil {
						return fmt.Errorf("error writing team locks to DB for team '%s' on '%s': %v",
							teamName, envName, err)
					}
				}
				if len(activeLockIds) == 0 {
					activeLockIds = []string{}
				}
				err := h.DBWriteAllTeamLocks(ctx, transaction, 0, envName, teamName, activeLockIds)
				if err != nil {
					return fmt.Errorf("error writing existing locks to DB for team '%s' on environment '%s': %v",
						teamName, envName, err)
				}
			}
		}
		return nil
	})
}

func (h *DBHandler) RunCustomMigrationQueuedApplicationVersions(ctx context.Context, getAllQueuedVersionsFun GetAllQueuedVersionsFun) error {
	return h.WithTransaction(ctx, false, func(ctx context.Context, transaction *sql.Tx) error {
		l := logger.FromContext(ctx).Sugar()
		allTeamLocksDb, err := h.DBSelectAnyDeploymentAttempt(ctx, transaction)
		if err != nil {
			l.Infof("could not get queued deployments friom database - assuming the manifest repo is correct: %v", err)
			allTeamLocksDb = nil
		}
		if allTeamLocksDb != nil {
			l.Infof("There are already queued deployments in the DB - skipping migrations")
			return nil
		}

		allQueuedVersionsInRepo, err := getAllQueuedVersionsFun(ctx)
		if err != nil {
			return fmt.Errorf("could not get current queued versions to run custom migrations: %v", err)
		}

		for envName, apps := range allQueuedVersionsInRepo {
			for appName, v := range apps {
				err := h.DBWriteDeploymentAttempt(ctx, transaction, envName, appName, v)
				if err != nil {
					return fmt.Errorf("error writing existing queued application version '%d' to DB for app '%s' on environment '%s': %v",
						*v, appName, envName, err)
				}
			}
		}
		return nil
	})
}
