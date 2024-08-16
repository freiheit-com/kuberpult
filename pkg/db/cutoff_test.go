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
	"testing"

	"github.com/freiheit-com/kuberpult/pkg/testutil"
	"github.com/google/go-cmp/cmp"
)

// For testing purposes only
type EmptyTransformer struct{}

func TestTransformerWritesEslDataRoundTrip(t *testing.T) {
	tcs := []struct {
		Name               string
		eslVersion         []EslVersion
		ExpectedEslVersion EslVersion
	}{
		{
			Name:               "test with one write operation",
			eslVersion:         []EslVersion{1},
			ExpectedEslVersion: 1,
		},
		{
			Name:               "test with multiple write operations",
			eslVersion:         []EslVersion{1, 2, 3, 4, 5},
			ExpectedEslVersion: 5,
		},
	}

	for _, tc := range tcs {
		tc := tc
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()
			ctx := testutil.MakeTestContext()

			dbHandler := setupDB(t)

			err := dbHandler.WithTransaction(ctx, false, func(ctx context.Context, transaction *sql.Tx) error {
				//We need to add transformers for these eslVersions beforehand (FK)
				tf := EmptyTransformer{}
				i := 0
				for i < len(tc.eslVersion) {
					//Write bogus transformer for FK reasons
					err := dbHandler.DBWriteEslEventInternal(ctx, "empty", transaction, interface{}(tf), ESLMetadata{})
					if err != nil {
						return err
					}
					i++
				}
				eslVersion, err2 := DBReadCutoff(dbHandler, ctx, transaction)
				if err2 != nil {
					return err2
				}
				if eslVersion != nil {
					return errors.New(fmt.Sprintf("expected no eslVersion, but got %v", *eslVersion))
				}

				for _, eslVersion := range tc.eslVersion {
					err := DBWriteCutoff(dbHandler, ctx, transaction, eslVersion)
					if err != nil {
						return err
					}
				}

				actual, err := DBReadCutoff(dbHandler, ctx, transaction)
				if err != nil {
					return err
				}

				if diff := cmp.Diff(tc.ExpectedEslVersion, *actual); diff != "" {
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
