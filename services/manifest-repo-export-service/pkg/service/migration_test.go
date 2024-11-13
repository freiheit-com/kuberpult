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
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"

	api "github.com/freiheit-com/kuberpult/pkg/api/v1"
)

func TestRunMigrations(t *testing.T) {
	type TestCase struct {
		name             string
		kuberpultVersion *api.KuberpultVersion
		expectedError    error
	}

	tcs := []TestCase{
		{
			name:             "empty migrations should succeed",
			kuberpultVersion: migrations.CreateKuberpultVersion(0, 1, 2),
			expectedError:    nil,
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

			err := migrationServer.RunMigrations(ctx, tc.kuberpultVersion)

			if diff := cmp.Diff(tc.expectedError, err, cmpopts.EquateErrors()); diff != "" {
				t.Errorf("error mismatch (-want, +got):\n%s", diff)
			}
		})
	}
}
