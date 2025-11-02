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

package reposerver

import (
	"context"
	"database/sql"
	"fmt"
	"regexp"
	"testing"

	"github.com/freiheit-com/kuberpult/pkg/types"

	"github.com/freiheit-com/kuberpult/pkg/db"
	"github.com/freiheit-com/kuberpult/pkg/testutil"

	v1alpha1 "github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
	argorepo "github.com/argoproj/argo-cd/v2/reposerver/apiclient"
	"github.com/freiheit-com/kuberpult/pkg/config"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"google.golang.org/protobuf/testing/protocmp"
)

var devEnvironment db.DBEnvironment = db.DBEnvironment{
	Name: "development",
	Config: config.EnvironmentConfig{
		Upstream: &config.EnvironmentConfigUpstream{
			Latest: true,
		},
		ArgoCd: &config.EnvironmentConfigArgoCd{
			Destination: config.ArgoCdDestination{
				Server: "development",
			},
		},
	},
}

var appRelease db.DBReleaseWithMetaData = db.DBReleaseWithMetaData{
	ReleaseNumbers: types.ReleaseNumbers{
		Revision: 0,
		Version:  &appVersion,
	},
	App: "app",
	Manifests: db.DBReleaseManifests{
		Manifests: map[types.EnvName]string{
			"development": `
api: v1
kind: ConfigMap
metadata:
  name: something
  namespace: something
data:
  key: value
---
api: v1
kind: ConfigMap
metadata:
  name: somethingelse
  namespace: somethingelse
data:
  key: value
`,
		},
	},
}

var appVersion uint64 = 1

func TestToRevision(t *testing.T) {
	tcs := []struct {
		Name           string
		ReleaseVersion uint64
		Expected       PseudoRevision
	}{
		{
			ReleaseVersion: 0,
			Expected:       "0000000000000000000000000000000000000000",
		},
		{
			ReleaseVersion: 666,
			Expected:       "0000000000000000000000000000000000000666",
		},
		{
			ReleaseVersion: 1234567890,
			Expected:       "0000000000000000000000000000001234567890",
		},
	}
	for _, tc := range tcs {
		tc := tc
		t.Run(fmt.Sprintf("TestToRevision_%v", tc.ReleaseVersion), func(t *testing.T) {
			{
				// one way test:
				actual := ToRevision(tc.ReleaseVersion)
				if diff := cmp.Diff(tc.Expected, actual); diff != "" {
					t.Errorf("response mismatch (-want, +got):\n%s", diff)
				}
			}
			{
				// round-trip test:
				actual, err := FromRevision(tc.Expected)
				if err != nil {
					t.Fatalf("FromRevision failed: %v", err)
				}
				if diff := cmp.Diff(tc.ReleaseVersion, actual); diff != "" {
					t.Errorf("response mismatch (-want, +got):\n%s", diff)
				}
			}

		})
	}
}

func TestGenerateManifest(t *testing.T) {
	tcs := []struct {
		Name              string
		SetupEnv          db.DBEnvironment
		SetupReleases     []db.DBReleaseWithMetaData
		Request           *argorepo.ManifestRequest
		ExpectedResponse  *argorepo.ManifestResponse
		ExpectedError     error
		ExpectedArgoError *regexp.Regexp
		DBOnlyTest        bool
	}{
		{
			Name:          "generates a manifest for HEAD",
			SetupEnv:      devEnvironment,
			SetupReleases: []db.DBReleaseWithMetaData{appRelease},
			Request: &argorepo.ManifestRequest{
				Revision: "HEAD",
				Repo: &v1alpha1.Repository{
					Repo: "<the-repo-url>",
				},
				ApplicationSource: &v1alpha1.ApplicationSource{
					Path: "environments/development/applications/app/manifests",
				},
			},

			ExpectedResponse: &argorepo.ManifestResponse{
				Manifests: []string{
					`{"api":"v1","data":{"key":"value"},"kind":"ConfigMap","metadata":{"name":"something","namespace":"something"}}`,
					`{"api":"v1","data":{"key":"value"},"kind":"ConfigMap","metadata":{"name":"somethingelse","namespace":"somethingelse"}}`,
				},
				SourceType: "Directory",
			},
		},
		{
			Name:          "generates a manifest for the branch itself",
			SetupEnv:      devEnvironment,
			SetupReleases: []db.DBReleaseWithMetaData{appRelease},
			Request: &argorepo.ManifestRequest{
				Revision: "master",
				Repo: &v1alpha1.Repository{
					Repo: "<the-repo-url>",
				},
				ApplicationSource: &v1alpha1.ApplicationSource{
					Path: "environments/development/applications/app/manifests",
				},
			},

			ExpectedResponse: &argorepo.ManifestResponse{
				Manifests: []string{
					`{"api":"v1","data":{"key":"value"},"kind":"ConfigMap","metadata":{"name":"something","namespace":"something"}}`,
					`{"api":"v1","data":{"key":"value"},"kind":"ConfigMap","metadata":{"name":"somethingelse","namespace":"somethingelse"}}`,
				},
				SourceType: "Directory",
			},
		},
	}
	for _, tc := range tcs {
		tc := tc
		t.Run(tc.Name+"_with_db", func(t *testing.T) {
			dbHandler := SetupRepositoryTestWithDBOptions(t, false)
			ctx := testutil.MakeTestContext()
			_ = dbHandler.WithTransaction(ctx, false, func(ctx context.Context, transaction *sql.Tx) error {
				err := dbHandler.DBWriteEnvironment(ctx, transaction, tc.SetupEnv.Name, tc.SetupEnv.Config, tc.SetupEnv.Applications)
				if err != nil {
					return err
				}

				for _, release := range tc.SetupReleases {
					err := dbHandler.DBInsertOrUpdateApplication(ctx, transaction, release.App, db.AppStateChangeCreate, db.DBAppMetaData{})
					if err != nil {
						return err
					}

					err = dbHandler.DBUpdateOrCreateRelease(ctx, transaction, release)
					if err != nil {
						return err
					}

					err = dbHandler.DBUpdateOrCreateDeployment(ctx, transaction, db.Deployment{
						App: release.App,
						Env: tc.SetupEnv.Name,
						ReleaseNumbers: types.ReleaseNumbers{
							Revision: 0,
							Version:  &appVersion,
						},
					})
					if err != nil {
						return err
					}
				}

				return nil
			})

			if tc.ExpectedResponse != nil {
				tc.ExpectedResponse.Revision = ToRevision(uint64(appVersion))
			}

			srv := New(dbHandler)
			resp, err := srv.GenerateManifest(context.Background(), tc.Request)
			if diff := cmp.Diff(tc.ExpectedError, err, cmpopts.EquateErrors()); diff != "" {
				t.Fatalf("error mismatch (-want, +got):\n%s", diff)
			}
			if diff := cmp.Diff(tc.ExpectedResponse, resp, protocmp.Transform()); diff != "" {
				t.Errorf("response mismatch (-want, +got):\n%s", diff)
			}

		})
	}
}

func TestGetRevisionMetadata(t *testing.T) {
	tcs := []struct {
		Name string
	}{
		{
			Name: "returns a dummy",
		},
	}
	for _, tc := range tcs {
		tc := tc
		t.Run(tc.Name, func(t *testing.T) {
			srv := (*reposerver)(nil)
			req := argorepo.RepoServerRevisionMetadataRequest{}
			_, err := srv.GetRevisionMetadata(
				context.Background(),
				&req,
			)
			if err != nil {
				t.Errorf("expected no error, but got %q", err)
			}
		})
	}
}

func SetupRepositoryTestWithDBOptions(t *testing.T, writeEslOnly bool) *db.DBHandler {
	ctx := context.Background()
	migrationsPath, err := db.CreateMigrationsPath(5)
	if err != nil {
		t.Fatalf("CreateMigrationsPath error: %v", err)
	}
	dbConfig, err := db.ConnectToPostgresContainer(ctx, t, migrationsPath, writeEslOnly, t.Name())
	if err != nil {
		t.Fatalf("SetupPostgres: %v", err)
	}

	migErr := db.RunDBMigrations(ctx, *dbConfig)
	if migErr != nil {
		t.Fatal(fmt.Errorf("path %s, error: %w", migrationsPath, migErr))
	}

	dbHandler, err := db.Connect(ctx, *dbConfig)
	if err != nil {
		t.Fatal(err)
	}

	return dbHandler
}
