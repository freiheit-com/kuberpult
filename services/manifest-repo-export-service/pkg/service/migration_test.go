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

package service

import (
	"context"
	"database/sql"
	"github.com/freiheit-com/kuberpult/pkg/migrations"
	"github.com/freiheit-com/kuberpult/pkg/testutil"
	migrations2 "github.com/freiheit-com/kuberpult/services/manifest-repo-export-service/pkg/migrations"
	"google.golang.org/protobuf/testing/protocmp"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"

	api "github.com/freiheit-com/kuberpult/pkg/api/v1"
)

// Used to compare two error message strings, needed because errors.Is(fmt.Errorf(text),fmt.Errorf(text)) == false
type errMatcher struct {
	msg string
}

func (e errMatcher) Error() string {
	return e.msg
}

func (e errMatcher) Is(err error) bool {
	return e.Error() == err.Error()
}

func TestRunMigrations(t *testing.T) {
	type TestCase struct {
		name                      string
		requestedKuberpultVersion *api.KuberpultVersion
		expectedResponse          *api.EnsureCustomMigrationAppliedResponse
		expectedError             error
	}

	tcs := []TestCase{
		{
			name:                      "empty migrations should succeed",
			requestedKuberpultVersion: migrations.CreateKuberpultVersion(0, 1, 2),
			expectedResponse:          &api.EnsureCustomMigrationAppliedResponse{MigrationsApplied: true},
			expectedError:             nil,
		},
		{
			name:                      "nil version should fail",
			requestedKuberpultVersion: nil,
			expectedResponse:          nil,
			expectedError:             errMatcher{msg: "requested kuberpult version is nil"},
		},
		{
			name:                      "different version in request vs env var => should fail",
			requestedKuberpultVersion: migrations.CreateKuberpultVersion(1, 2, 3),
			expectedResponse:          nil,
			expectedError:             errMatcher{msg: "different versions of kuberpult are running: 1.2.3!=0.1.2"},
		},
	}

	for _, tc := range tcs {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			repo, _ := setupRepositoryTestWithPath(t)

			ctx := testutil.MakeTestContext()
			dbHandler := repo.State().DBHandler

			migrationServer := MigrationServer{
				DBHandler:        dbHandler,
				Migrations:       []*Migration{},
				KuberpultVersion: migrations.CreateKuberpultVersion(0, 1, 2),
			}

			response, err := migrationServer.EnsureCustomMigrationApplied(
				ctx,
				&api.EnsureCustomMigrationAppliedRequest{Version: tc.requestedKuberpultVersion},
			)

			if diff := cmp.Diff(tc.expectedResponse, response, protocmp.Transform()); diff != "" {
				t.Errorf("response mismatch (-want, +got):\n%s", diff)
			}
			if diff := cmp.Diff(tc.expectedError, err, cmpopts.EquateErrors()); diff != "" {
				t.Errorf("error mismatch (-want, +got):\n%s", diff)
			}
		})
	}
}

func TestMigrationDetails(t *testing.T) {
	type TestCase struct {
		name                       string
		configuredKuberpultVersion *api.KuberpultVersion // this is what is usually configured as env var
		requestedKuberpultVersion  *api.KuberpultVersion // how we call the function EnsureCustomMigrationApplied
		allMigrations              []*Migration
		dbVersions                 []*api.KuberpultVersion // all versions that are written to db before the test
		expectedDbVersions         []*api.KuberpultVersion // all versions that are written to db before the test
		expectedResponse           *api.EnsureCustomMigrationAppliedResponse
		expectedError              error
		expectedWasRun             bool
	}

	var wasRun = false
	var v100 = migrations.CreateKuberpultVersion(1, 0, 0)
	var v200 = migrations.CreateKuberpultVersion(2, 0, 0)
	tcs := []TestCase{
		{
			name:                       "migration is executed, even though db has newer version",
			configuredKuberpultVersion: v100,
			requestedKuberpultVersion:  v100,
			dbVersions: []*api.KuberpultVersion{
				v200,
			},
			allMigrations: []*Migration{
				{
					Version: v100,
					Migration: func(ctx context.Context) error {
						wasRun = true
						return nil
					},
				},
			},
			expectedResponse: &api.EnsureCustomMigrationAppliedResponse{MigrationsApplied: true},
			expectedError:    nil,
			expectedWasRun:   true,
			expectedDbVersions: []*api.KuberpultVersion{
				v200,
				v100,
			},
		},
		{
			name:                       "do not run migration, if db has that migration already",
			configuredKuberpultVersion: v100,
			requestedKuberpultVersion:  v100,
			dbVersions: []*api.KuberpultVersion{
				v100,
			},
			allMigrations: []*Migration{
				{
					Version: v100,
					Migration: func(ctx context.Context) error {
						wasRun = true
						return nil
					},
				},
			},
			expectedResponse:   &api.EnsureCustomMigrationAppliedResponse{MigrationsApplied: true},
			expectedError:      nil,
			expectedWasRun:     false,
			expectedDbVersions: []*api.KuberpultVersion{},
		},
	}

	for _, tc := range tcs {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			wasRun = false

			repo, _ := setupRepositoryTestWithPath(t)

			ctx := testutil.MakeTestContext()
			dbHandler := repo.State().DBHandler

			migrationServer := MigrationServer{
				KuberpultVersion: tc.configuredKuberpultVersion,
				DBHandler:        dbHandler,
				Migrations:       tc.allMigrations,
			}

			err := dbHandler.WithTransaction(ctx, false, func(ctx context.Context, transaction *sql.Tx) error {
				for _, dbVersion := range tc.dbVersions {
					err := migrations2.DBUpsertCustomMigrationCutoff(dbHandler, ctx, transaction, dbVersion)
					if err != nil {
						return err
					}
				}
				return nil
			})

			response, err := migrationServer.EnsureCustomMigrationApplied(ctx,
				&api.EnsureCustomMigrationAppliedRequest{Version: tc.requestedKuberpultVersion})

			if diff := cmp.Diff(tc.expectedResponse, response, protocmp.Transform()); diff != "" {
				t.Errorf("response mismatch (-want, +got):\n%s", diff)
			}
			if diff := cmp.Diff(tc.expectedError, err, cmpopts.EquateErrors()); diff != "" {
				t.Errorf("error mismatch (-want, +got):\n%s", diff)
			}
			if diff := cmp.Diff(tc.expectedWasRun, wasRun); diff != "" {
				t.Errorf("error mismatch (-want, +got):\n%s", diff)
			}
		})
	}
}
