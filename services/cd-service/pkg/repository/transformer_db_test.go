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

package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"github.com/freiheit-com/kuberpult/services/cd-service/pkg/repository/testutil"
	"google.golang.org/protobuf/testing/protocmp"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestTransformerWritesEslDataRoundtrip(t *testing.T) {
	tcs := []struct {
		Name              string
		Transformers      Transformer
		expectedEventJson string
	}{
		{
			Name: "Delete non-existent application",
			Transformers: &CreateEnvironmentLock{
				Authentication: Authentication{},
				Environment:    "dev",
				LockId:         "lock123",
				Message:        "msg321",
			},
		},
	}
	for _, tc := range tcs {
		tc := tc
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()
			ctx := testutil.MakeTestContext()
			cfg := DBConfig{
				MigrationsPath: "/kp/cd_database/migrations",
				todo: something wrong with the file path here
				DriverName:     "sqlite3",
			}
			repo, err := setupRepositoryTestWithDB(t, &cfg)
			if err != nil {
				t.Errorf("setup error\n%v", err)
			}
			r := repo.(*repository)
			row := EslEventRow{}
			err = r.DB.WithTransaction(ctx, func(ctx context.Context, transaction *sql.Tx) error {
				_, _, _, err2 := repo.ApplyTransformersInternal(testutil.MakeTestContext(), transaction, tc.Transformers)
				if err2 != nil {
					return err2
				}
				row, err3 := r.DB.DBReadEslEventInternal(ctx, transaction)
				if err3 != nil {
					return err3
				}
				if row == nil && err3 == nil {
					return errors.New("expected at least one row, but got 0")
				}
				return nil
			})
			//createEnvLock := tc.Transformers.(*CreateEnvironmentLock)
			var outputEnvLock *CreateEnvironmentLock = &CreateEnvironmentLock{}
			err = json.Unmarshal(([]byte)(row.EventJson), &outputEnvLock)
			if err != nil {
				t.Errorf("marshal error\n%v", err)
			}

			if diff := cmp.Diff(tc.Transformers, outputEnvLock, protocmp.Transform()); diff != "" {
				t.Fatalf("error mismatch (-want, +got):\n%s", diff)
			}

		})
	}
}
