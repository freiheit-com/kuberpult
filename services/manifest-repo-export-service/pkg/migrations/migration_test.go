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

package migrations

import (
	"context"
	"database/sql"
	"github.com/freiheit-com/kuberpult/pkg/db"
	"github.com/freiheit-com/kuberpult/pkg/testutil"
	"github.com/freiheit-com/kuberpult/services/manifest-repo-export-service/pkg/repository"
	"google.golang.org/protobuf/testing/protocmp"
	"os/exec"
	"path"
	"testing"

	api "github.com/freiheit-com/kuberpult/pkg/api/v1"
	"github.com/google/go-cmp/cmp"
)

func TestRunMigrations(t *testing.T) {
	type TestCase struct {
		name                     string
		kuberpultVersionToInsert *api.KuberpultVersion
		kuberpultVersionToQuery  *api.KuberpultVersion
		expectedVersion          *api.KuberpultVersion
		expectedError            error
	}

	tcs := []TestCase{
		{
			name:                     "should read nothing if nothing was written",
			kuberpultVersionToInsert: nil,
			kuberpultVersionToQuery:  CreateKuberpultVersion(0, 1, 2),
			expectedVersion:          nil,
			expectedError:            nil,
		},
		{
			name:                     "should read nothing if a different version was written",
			kuberpultVersionToInsert: CreateKuberpultVersion(2, 3, 4),
			kuberpultVersionToQuery:  CreateKuberpultVersion(0, 1, 2),
			expectedVersion:          nil,
			expectedError:            nil,
		},
		{
			name:                     "should read the same version that was written",
			kuberpultVersionToInsert: CreateKuberpultVersion(0, 1, 2),
			kuberpultVersionToQuery:  CreateKuberpultVersion(0, 1, 2),
			expectedVersion:          CreateKuberpultVersion(0, 1, 2),
			expectedError:            nil,
		},
	}

	for _, tc := range tcs {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			repo, _ := setupRepositoryTestWithPath(t)

			ctx := testutil.MakeTestContext()
			dbHandler := repo.State().DBHandler

			_ = dbHandler.WithTransaction(ctx, false, func(ctx context.Context, transaction *sql.Tx) error {

				if tc.kuberpultVersionToInsert != nil {
					err := DBWriteCustomMigrationCutoff(dbHandler, ctx, transaction, tc.kuberpultVersionToInsert)
					if err != nil {
						t.Fatal("unexpected error when writing cutoff: %w", err)
					}
				}

				returnedVersion, err := DBReadCustomMigrationCutoff(dbHandler, ctx, transaction, tc.kuberpultVersionToQuery)

				if err != nil {
					t.Fatal("unexpected error: %w", err)
				}
				if diff := cmp.Diff(tc.expectedVersion, returnedVersion, protocmp.Transform()); diff != "" {
					t.Errorf("error mismatch (-want, +got):\n%s", diff)
				}
				return nil
			})
		})
	}
}

func setupRepositoryTestWithPath(t *testing.T) (repository.Repository, string) {
	ctx := context.Background()
	migrationsPath, err := testutil.CreateMigrationsPath(4)
	if err != nil {
		t.Fatalf("CreateMigrationsPath error: %v", err)
	}
	dbConfig := &db.DBConfig{
		MigrationsPath: migrationsPath,
		DriverName:     "sqlite3",
	}

	dir := t.TempDir()
	remoteDir := path.Join(dir, "remote")
	localDir := path.Join(dir, "local")
	cmd := exec.Command("git", "init", "--bare", remoteDir)
	err = cmd.Start()
	if err != nil {
		t.Errorf("could not start git init")
		return nil, ""
	}
	err = cmd.Wait()
	if err != nil {
		t.Errorf("could not wait for git init to finish")
		return nil, ""
	}

	repoCfg := repository.RepositoryConfig{
		URL:                  remoteDir,
		Path:                 localDir,
		CommitterEmail:       "kuberpult@freiheit.com",
		CommitterName:        "kuberpult",
		ArgoCdGenerateFiles:  true,
		ReleaseVersionLimit:  2,
		MinimizeExportedData: false,
	}

	if dbConfig != nil {
		dbConfig.DbHost = dir

		migErr := db.RunDBMigrations(ctx, *dbConfig)
		if migErr != nil {
			t.Fatal(migErr)
		}

		dbHandler, err := db.Connect(ctx, *dbConfig)
		if err != nil {
			t.Fatal(err)
		}
		repoCfg.DBHandler = dbHandler
	}

	repo, err := repository.New(
		testutil.MakeTestContext(),
		repoCfg,
	)
	if err != nil {
		t.Fatal(err)
	}
	return repo, remoteDir
}
