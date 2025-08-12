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
	"github.com/freiheit-com/kuberpult/pkg/config"
	"github.com/freiheit-com/kuberpult/pkg/db"
	"github.com/freiheit-com/kuberpult/pkg/testutil"
	"github.com/freiheit-com/kuberpult/pkg/types"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"testing"
	"time"
)

func TestDBSelectAppsWithDeploymentInEnvAtTimestamp(t *testing.T) {
	const appFoo = types.AppName("foo")
	const appPow = types.AppName("pow")
	const dev = types.EnvName("dev")
	const stg = types.EnvName("staging")

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
			Manifests:      db.DBReleaseManifests{Manifests: map[types.EnvName]string{dev: "manifest1", "staging": "manifest2"}},
		},
		{
			ReleaseNumbers: types.MakeReleaseNumberVersion(2),
			App:            appFoo,
			Manifests:      db.DBReleaseManifests{Manifests: map[types.EnvName]string{dev: "manifest1", "staging": "manifest2"}},
		},
		{
			ReleaseNumbers: types.MakeReleaseNumberVersion(1),
			App:            appPow,
			Manifests:      db.DBReleaseManifests{Manifests: map[types.EnvName]string{dev: "manifest1", "staging": "manifest2"}},
		},
		{
			ReleaseNumbers: types.MakeReleaseNumberVersion(2),
			App:            appPow,
			Manifests:      db.DBReleaseManifests{Manifests: map[types.EnvName]string{dev: "manifest1", "staging": "manifest2"}},
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
		ExpectedDeployments map[types.EnvName]DeploymentMap
	}{
		{
			Name: "one simple deployment works",
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
						Created:        time.Time{},
						App:            string(appFoo),
						Env:            dev,
						ReleaseNumbers: types.MakeReleaseNumbers(1, 0),
						Metadata:       db.DeploymentMetadata{},
						TransformerID:  0,
					},
				},
			},
		},
		{
			Name: "un-deploying works",
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
						Created:        time.Time{},
						App:            string(appFoo),
						Env:            dev,
						ReleaseNumbers: types.MakeReleaseNumbers(0, 0),
						Metadata:       db.DeploymentMetadata{},
						TransformerID:  0,
					},
				},
			},
		},
		{
			Name: "re-deploying works",
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
						Created:        time.Time{},
						App:            string(appFoo),
						Env:            dev,
						ReleaseNumbers: types.MakeReleaseNumbers(1, 0),
						Metadata:       db.DeploymentMetadata{},
						TransformerID:  0,
					},
				},
			},
		},
		{
			Name: "two simple deployments works",
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
						Created:        time.Time{},
						App:            string(appFoo),
						Env:            dev,
						ReleaseNumbers: types.MakeReleaseNumbers(1, 0),
						Metadata:       db.DeploymentMetadata{},
						TransformerID:  0,
					},
				},
				stg:{
					appPow: {
						Created:        time.Time{},
						App:            string(appPow),
						Env:            stg,
						ReleaseNumbers: types.MakeReleaseNumbers(1, 0),
						Metadata:       db.DeploymentMetadata{},
						TransformerID:  0,
					},
				},
			},
		},
	}

	for _, tc := range tcs {
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()
			ctx := testutil.MakeTestContext()
			dbHandler := setupDB(t)
			var err error
			err = dbHandler.WithTransaction(ctx, false, func(ctx context.Context, transaction *sql.Tx) error {
				// GIVEN:
				for _, environment := range Environments {
					err := dbHandler.DBWriteEnvironment(ctx, transaction, environment.Name, environment.Config, make([]string, 0))
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
						App:            string(toBeDeployed.App),
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
					actualResult, err := DBSelectAppsWithDeploymentInEnvAtTimestamp(ctx, dbHandler, transaction, envName, *timestamp)
					if err != nil {
						t.Fatalf("error selecting deployments: %v", err)
					}
					// THEN:
					if diff := cmp.Diff(expectedDeploymentMap, actualResult, cmpopts.IgnoreFields(db.Deployment{}, "Created")); diff != "" {
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

	dbConfig, err := db.ConnectToPostgresContainer(ctx, t, dir, false, t.Name())
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
