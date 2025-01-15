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
	"github.com/freiheit-com/kuberpult/pkg/migrations"
	"github.com/freiheit-com/kuberpult/pkg/testutil"
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
		name             string
		kuberpultVersion *api.KuberpultVersion
		expectedResponse *api.EnsureCustomMigrationAppliedResponse
		expectedError    error
	}

	tcs := []TestCase{
		{
			name:             "empty migrations should succeed",
			kuberpultVersion: migrations.CreateKuberpultVersion(0, 1, 2),
			expectedResponse: &api.EnsureCustomMigrationAppliedResponse{MigrationsApplied: true},
			expectedError:    nil,
		},
		{
			name:             "nil version should fail",
			kuberpultVersion: nil,
			expectedResponse: nil,
			expectedError:    errMatcher{msg: "kuberpult version is nil"},
		},
	}

	for _, tc := range tcs {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			repo, _ := setupRepositoryTestWithPath(t)

			ctx := testutil.MakeTestContext()
			dbHandler := repo.State().DBHandler

			migrationServer := MigrationServer{
				DBHandler: dbHandler,
			}

			response, err := migrationServer.EnsureCustomMigrationApplied(ctx, &api.EnsureCustomMigrationAppliedRequest{Version: tc.kuberpultVersion})

			if diff := cmp.Diff(tc.expectedResponse, response, protocmp.Transform()); diff != "" {
				t.Errorf("response mismatch (-want, +got):\n%s", diff)
			}
			if diff := cmp.Diff(tc.expectedError, err, cmpopts.EquateErrors()); diff != "" {
				t.Errorf("error mismatch (-want, +got):\n%s", diff)
			}
		})
	}
}
