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

package cutoff

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"github.com/freiheit-com/kuberpult/pkg/db"
	"github.com/freiheit-com/kuberpult/pkg/testutil"
	"github.com/google/go-cmp/cmp"
	"testing"
)

func TestTransformerWritesEslDataRoundTrip(t *testing.T) {
	tcs := []struct {
		Name          string
		eslId         []db.EslId
		ExpectedEslId db.EslId
	}{
		{
			Name:          "test with one write operation",
			eslId:         []db.EslId{7},
			ExpectedEslId: 7,
		},
		{
			Name:          "test with multiple write operations",
			eslId:         []db.EslId{1, 2, 7, 666, 777},
			ExpectedEslId: 777,
		},
	}

	for _, tc := range tcs {
		tc := tc
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()
			ctx := testutil.MakeTestContext()

			dbHandler := setupDB(t)

			err := dbHandler.WithTransaction(ctx, func(ctx context.Context, transaction *sql.Tx) error {
				eslId, err2 := DBReadCutoff(dbHandler, ctx, transaction)
				if err2 != nil {
					return err2
				}
				if eslId != nil {
					return errors.New(fmt.Sprintf("expected no eslId, but got %v", *eslId))
				}

				for _, eslId := range tc.eslId {
					err := DBWriteCutoff(dbHandler, ctx, transaction, eslId)
					if err != nil {
						return err
					}
				}

				actual, err := DBReadCutoff(dbHandler, ctx, transaction)
				if err != nil {
					return err
				}

				if diff := cmp.Diff(tc.ExpectedEslId, *actual); diff != "" {
					t.Fatalf("error mismatch (-want, +got):\n%s", diff)
				}
				return nil
			})
			if err != nil {
				t.Fatalf("transaction error: %v", err)
			}
		})
	}
}

// setupDB returns a new DBHandler with a tmp directory every time, so tests can are completely independent
func setupDB(t *testing.T) *db.DBHandler {
	dir, err := testutil.CreateMigrationsPath()
	tmpDir := t.TempDir()
	t.Logf("directory for DB migrations: %s", dir)
	t.Logf("tmp dir for DB data: %s", tmpDir)
	cfg := db.DBConfig{
		MigrationsPath: dir,
		DriverName:     "sqlite3",
		DbHost:         tmpDir,
	}

	migErr := db.RunDBMigrations(cfg)
	if migErr != nil {
		t.Fatal(migErr)
	}

	dbHandler, err := db.Connect(cfg)
	if err != nil {
		t.Fatal(err)
	}

	return dbHandler
}
