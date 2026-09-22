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

package db_history

import (
	"context"
	"database/sql"
	"fmt"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp/cmpopts"

	"github.com/freiheit-com/kuberpult/pkg/config"
	"github.com/freiheit-com/kuberpult/pkg/db"
	"github.com/freiheit-com/kuberpult/pkg/testutil"
	"github.com/freiheit-com/kuberpult/pkg/testutilauth"
	"github.com/freiheit-com/kuberpult/pkg/types"
)

func TestDBSelectAppsWithDeploymentInEnvAtTimestamp(t *testing.T) {
	const appFoo = types.AppName("foo")
	const appPow = types.AppName("pow")
	allApps := []types.AppName{appFoo, appPow}
	allAppsDuplicated := []types.AppName{appFoo, appPow, appFoo, appPow}

	const dev = types.EnvName("dev")
	const stg = types.EnvName("staging")
	allEnvs := []types.EnvName{dev, stg}

	Environments := []db.DBEnvironment{
		{
			Name:   dev,
			Config: config.EnvironmentConfig{},
		},
		{
			Name:   stg,
			Config: config.EnvironmentConfig{},
		},
	}
	Releases := []db.DBReleaseWithMetaData{
		{
			ReleaseNumbers: types.MakeReleaseNumberVersion(1),
			App:            appFoo,
			Manifests:      db.DBReleaseManifests{Manifests: map[types.EnvName]string{dev: "manifest1", stg: "manifest2"}},
		},
		{
			ReleaseNumbers: types.MakeReleaseNumberVersion(2),
			App:            appFoo,
			Manifests:      db.DBReleaseManifests{Manifests: map[types.EnvName]string{dev: "manifest1", stg: "manifest2"}},
		},
		{
			ReleaseNumbers: types.MakeReleaseNumberVersion(1),
			App:            appPow,
			Manifests:      db.DBReleaseManifests{Manifests: map[types.EnvName]string{dev: "manifest1", stg: "manifest2"}},
		},
		{
			ReleaseNumbers: types.MakeReleaseNumberVersion(2),
			App:            appPow,
			Manifests:      db.DBReleaseManifests{Manifests: map[types.EnvName]string{dev: "manifest1", stg: "manifest2"}},
		},
		{
			ReleaseNumbers: types.MakeReleaseNumberVersion(0),
			App:            appFoo,
			Manifests:      db.DBReleaseManifests{Manifests: map[types.EnvName]string{dev: "manifest1", stg: "manifest2"}},
		},
	}

	type AppEnv struct {
		App            types.AppName
		Env            types.EnvName
		ReleaseNumbers types.ReleaseNumbers
	}

	tcs := []struct {
		Name                string
		InputDeployments    []AppEnv
		InputAllApps        []types.AppName
		ExpectedDeployments map[types.EnvName]DeploymentMap
	}{
		{
			Name:         "one simple deployment works and appPow is skipped",
			InputAllApps: allApps,
			InputDeployments: []AppEnv{
				{
					App:            appFoo,
					Env:            dev,
					ReleaseNumbers: types.MakeReleaseNumbers(1, 0),
				},
			},
			ExpectedDeployments: map[types.EnvName]DeploymentMap{
				dev: {
					appFoo: {
						ReleaseVersion: types.Ptr(uint64(1)),
						Revision:       0,
					},
				},
				stg: {},
			},
		},
		{
			Name:         "huge list of apps works",
			InputAllApps: createHugeListOfApps(70_000), // a "where-in" would be limited to 65k
			InputDeployments: []AppEnv{
				{
					App:            appFoo,
					Env:            dev,
					ReleaseNumbers: types.MakeReleaseNumbers(1, 0),
				},
			},
			ExpectedDeployments: map[types.EnvName]DeploymentMap{
				dev: {},
				stg: {},
			},
		},
		{
			Name:         "un-deploying works",
			InputAllApps: allApps,
			InputDeployments: []AppEnv{
				{
					App:            appFoo,
					Env:            dev,
					ReleaseNumbers: types.MakeReleaseNumbers(1, 0),
				},
				{
					App:            appFoo,
					Env:            dev,
					ReleaseNumbers: types.MakeReleaseNumbers(0, 0),
				},
			},
			ExpectedDeployments: map[types.EnvName]DeploymentMap{
				dev: {
					appFoo: {
						ReleaseVersion: types.Ptr(uint64(0)),
						Revision:       0,
					},
				},
				stg: {},
			},
		},
		{
			Name:         "re-deploying works",
			InputAllApps: allApps,
			InputDeployments: []AppEnv{
				{
					App:            appFoo,
					Env:            dev,
					ReleaseNumbers: types.MakeReleaseNumbers(1, 0),
				},
				{
					App:            appFoo,
					Env:            dev,
					ReleaseNumbers: types.MakeReleaseNumbers(0, 0),
				},
				{
					App:            appFoo,
					Env:            dev,
					ReleaseNumbers: types.MakeReleaseNumbers(1, 0),
				},
			},
			ExpectedDeployments: map[types.EnvName]DeploymentMap{
				dev: {
					appFoo: {
						ReleaseVersion: types.Ptr(uint64(1)),
						Revision:       0,
					},
				},
				stg: {},
			},
		},
		{
			Name:         "two simple deployments works",
			InputAllApps: allApps,
			InputDeployments: []AppEnv{
				{
					App:            appFoo,
					Env:            dev,
					ReleaseNumbers: types.MakeReleaseNumbers(1, 0),
				},
				{
					App:            appPow,
					Env:            stg,
					ReleaseNumbers: types.MakeReleaseNumbers(1, 0),
				},
			},
			ExpectedDeployments: map[types.EnvName]DeploymentMap{
				dev: {
					appFoo: {
						ReleaseVersion: types.Ptr(uint64(1)),
						Revision:       0,
					},
				},
				stg: {
					appPow: {
						ReleaseVersion: types.Ptr(uint64(1)),
						Revision:       0,
					},
				},
			},
		},
		{
			Name:         "empty apps return nothing",
			InputAllApps: []types.AppName{},
			InputDeployments: []AppEnv{
				{
					App:            appFoo,
					Env:            dev,
					ReleaseNumbers: types.MakeReleaseNumbers(1, 0),
				},
				{
					App:            appPow,
					Env:            stg,
					ReleaseNumbers: types.MakeReleaseNumbers(1, 0),
				},
			},
			ExpectedDeployments: map[types.EnvName]DeploymentMap{
				dev: {},
				stg: {},
			},
		},
		{
			Name:         "duplicate input apps yield same result as unique list of apps",
			InputAllApps: allAppsDuplicated,
			InputDeployments: []AppEnv{
				{
					App:            appFoo,
					Env:            dev,
					ReleaseNumbers: types.MakeReleaseNumbers(1, 0),
				},
				{
					App:            appPow,
					Env:            stg,
					ReleaseNumbers: types.MakeReleaseNumbers(1, 0),
				},
			},
			ExpectedDeployments: map[types.EnvName]DeploymentMap{
				dev: {
					appFoo: {
						ReleaseVersion: types.Ptr(uint64(1)),
						Revision:       0,
					},
				},
				stg: {
					appPow: {
						ReleaseVersion: types.Ptr(uint64(1)),
						Revision:       0,
					},
				},
			},
		},
	}

	for _, tc := range tcs {
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()
			ctx := testutilauth.MakeTestContext()
			for _, env := range allEnvs {
				if tc.ExpectedDeployments[env] == nil {
					t.Fatalf("test setup broken: no deployment found for env %v: %s", env,
						"you need to specify all envs in 'ExpectedDeployments'")
				}
			}
			dbHandler := setupDB(t)
			var err error
			err = dbHandler.WithTransaction(ctx, false, func(ctx context.Context, transaction *sql.Tx) error {
				// GIVEN:
				for _, environment := range Environments {
					err := dbHandler.DBWriteEnvironment(ctx, transaction, environment.Name, environment.Config)
					if err != nil {
						t.Fatalf("error while writing environment, error: %v", err)
					}
				}
				for _, release := range Releases {
					err := dbHandler.DBUpdateOrCreateRelease(ctx, transaction, release)
					if err != nil {
						t.Fatalf("error while writing release, error: %v", err)
					}
				}
				return nil
			})
			if err != nil {
				t.Fatalf("error: %v", err)
			}
			for _, toBeDeployed := range tc.InputDeployments {
				// starting new transactions here so that each deployment gets its own timestamp
				err = dbHandler.WithTransaction(ctx, false, func(ctx context.Context, transaction *sql.Tx) error {

					err := dbHandler.DBUpdateOrCreateDeployment(ctx, transaction, db.Deployment{
						Created:        time.Time{},
						App:            toBeDeployed.App,
						Env:            toBeDeployed.Env,
						ReleaseNumbers: toBeDeployed.ReleaseNumbers,
						Metadata:       db.DeploymentMetadata{},
						TransformerID:  0,
					})
					if err != nil {
						t.Fatalf("error writing deployment %v: %v", toBeDeployed, err)
					}
					return nil
				})
				if err != nil {
					t.Fatalf("error: %v", err)
				}
			}
			err = dbHandler.WithTransaction(ctx, false, func(ctx context.Context, transaction *sql.Tx) error {
				// WHEN:
				timestamp, err := dbHandler.DBReadTransactionTimestamp(ctx, transaction)
				if err != nil {
					t.Fatalf("error getting time: %v", err)
				}
				for envName, expectedDeploymentMap := range tc.ExpectedDeployments {
					actualResult, err := DBSelectAppsWithDeploymentInEnvAtTimestamp(ctx, transaction, envName, *timestamp, tc.InputAllApps)
					if err != nil {
						t.Fatalf("error selecting deployments: %v", err)
					}
					// THEN:
					if diff := testutil.CmpDiff(expectedDeploymentMap, actualResult, cmpopts.IgnoreFields(db.Deployment{}, "Created")); diff != "" {
						t.Fatalf("deployment mismatch on env %s (-want, +got):\n%s", envName, diff)
					}
				}
				return nil
			})
			if err != nil {
				t.Fatalf("error: %v", err)
			}
		})
	}
}

func createHugeListOfApps(numApps uint) []types.AppName {
	var results []types.AppName
	for i := range numApps {
		results = append(results, types.AppName(fmt.Sprintf("TmpApp-%d", i)))
	}
	return results
}

// TestDBDeleteDeploymentWithHistoryReflectsInTimestampQuery ensures that deleting a deployment via
// the history-aware wrapper makes the timestamped history query stop reporting the app as deployed.
// A plain DBDeleteDeployment (current table only) would leave the history showing the old version,
// which is the bug that kept deleted apps in the env's argocd root app.
func TestDBDeleteDeploymentWithHistoryReflectsInTimestampQuery(t *testing.T) {
	const app = types.AppName("foo")
	const dev = types.EnvName("dev")
	allApps := []types.AppName{app}
	tcs := []struct {
		Name          string
		DeployVersion uint64
	}{
		{
			Name:          "deleting a deployment marks it un-deployed in history",
			DeployVersion: 1,
		},
	}
	for _, tc := range tcs {
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()
			ctx := testutilauth.MakeTestContext()
			dbHandler := setupDB(t)

			// GIVEN: an app deployed to dev
			err := dbHandler.WithTransaction(ctx, false, func(ctx context.Context, transaction *sql.Tx) error {
				if err := dbHandler.DBWriteEnvironment(ctx, transaction, dev, config.EnvironmentConfig{}); err != nil {
					return err
				}
				if err := dbHandler.DBUpdateOrCreateRelease(ctx, transaction, db.DBReleaseWithMetaData{
					ReleaseNumbers: types.MakeReleaseNumberVersion(tc.DeployVersion),
					App:            app,
					Manifests:      db.DBReleaseManifests{Manifests: map[types.EnvName]string{dev: "manifest"}},
				}); err != nil {
					return err
				}
				return dbHandler.DBUpdateOrCreateDeployment(ctx, transaction, db.Deployment{
					Created:        time.Time{},
					App:            app,
					Env:            dev,
					ReleaseNumbers: types.MakeReleaseNumbers(tc.DeployVersion, 0),
					Metadata:       db.DeploymentMetadata{},
					TransformerID:  0,
				})
			})
			if err != nil {
				t.Fatalf("setup: %v", err)
			}

			// sanity check: the app is reported as deployed before deletion
			err = dbHandler.WithTransaction(ctx, false, func(ctx context.Context, transaction *sql.Tx) error {
				ts, err := dbHandler.DBReadTransactionTimestamp(ctx, transaction)
				if err != nil {
					return err
				}
				res, err := DBSelectAppsWithDeploymentInEnvAtTimestamp(ctx, transaction, dev, *ts, allApps)
				if err != nil {
					return err
				}
				if dep, ok := res[app]; !ok || dep.ReleaseVersion == nil {
					t.Fatalf("expected app %q to be deployed before deletion, got: %v", app, res)
				}
				return nil
			})
			if err != nil {
				t.Fatalf("sanity: %v", err)
			}

			// WHEN: the deployment is deleted via the history-aware wrapper
			err = dbHandler.WithTransaction(ctx, false, func(ctx context.Context, transaction *sql.Tx) error {
				return dbHandler.DBDeleteDeploymentWithHistory(ctx, transaction, app, dev, 0, db.DeploymentMetadata{})
			})
			if err != nil {
				t.Fatalf("delete: %v", err)
			}

			// THEN: the timestamped history query no longer reports the app as deployed
			err = dbHandler.WithTransaction(ctx, false, func(ctx context.Context, transaction *sql.Tx) error {
				ts, err := dbHandler.DBReadTransactionTimestamp(ctx, transaction)
				if err != nil {
					return err
				}
				res, err := DBSelectAppsWithDeploymentInEnvAtTimestamp(ctx, transaction, dev, *ts, allApps)
				if err != nil {
					return err
				}
				if dep, ok := res[app]; ok && dep.ReleaseVersion != nil {
					t.Errorf("expected app %q to be un-deployed after deletion, but it still has version %d", app, *dep.ReleaseVersion)
				}
				return nil
			})
			if err != nil {
				t.Fatalf("verify: %v", err)
			}
		})
	}
}

// setupDB returns a new DBHandler with a tmp directory every time, so tests are completely independent
func setupDB(t *testing.T) *db.DBHandler {
	ctx := context.Background()
	dir, err := db.CreateMigrationsPath(4)
	if err != nil {
		t.Fatalf("CreateMigrationsPath: %v", err)
	}
	tmpDir := t.TempDir()
	t.Logf("directory for DB migrations: %s", dir)
	t.Logf("tmp dir for DB data: %s", tmpDir)

	dbConfig, err := db.ConnectToPostgresContainer(ctx, t, dir, t.Name())
	if err != nil {
		t.Fatalf("SetupPostgres: %v", err)
	}

	migErr := db.RunDBMigrations(ctx, *dbConfig)
	if migErr != nil {
		t.Fatal(migErr)
	}

	dbHandler, err := db.Connect(ctx, *dbConfig)
	if err != nil {
		t.Fatal(err)
	}

	return dbHandler
}
