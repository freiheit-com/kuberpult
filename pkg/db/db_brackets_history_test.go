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
	"fmt"
	"testing"
	"time"

	"github.com/freiheit-com/kuberpult/pkg/testutil"
	"github.com/freiheit-com/kuberpult/pkg/testutilauth"
	"github.com/freiheit-com/kuberpult/pkg/types"
)

func TestSelectBracketsHistoryByTimestamp(t *testing.T) {
	timeA := func() time.Time { return time.Time{} }
	tcs := []struct {
		Name                string
		PreparedBracketRows []BracketRow
		PreparedTimestamp   time.Time
		ExpectedBracketRow  *BracketRow
		ExpectedErr         error
	}{
		{
			Name:                "no data",
			PreparedBracketRows: []BracketRow{},
			PreparedTimestamp:   timeA(),
			ExpectedBracketRow:  nil,
		},
		{
			Name: "just one result",
			PreparedBracketRows: []BracketRow{
				{
					CreatedAt: timeA(),
					AllBracketsJsonBlob: BracketJsonBlob{
						BracketMap: map[types.ArgoBracketName]AppNames{
							"b1": {"app1", "app2"},
						},
					},
				},
			},
			PreparedTimestamp: timeA(),
			ExpectedBracketRow: &BracketRow{
				CreatedAt: timeA(),
				AllBracketsJsonBlob: BracketJsonBlob{
					BracketMap: map[types.ArgoBracketName]AppNames{
						"b1": {"app1", "app2"},
					},
				},
			},
		},
	}

	for _, tc := range tcs {
		tc := tc
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()
			ctx := testutilauth.MakeTestContext()
			dbHandler := setupDB(t)

			err := dbHandler.WithTransaction(ctx, false, func(ctx context.Context, transaction *sql.Tx) error {
				for _, bracketRow := range tc.PreparedBracketRows {
					err := DBInsertBracketHistory(dbHandler, ctx, transaction, bracketRow)
					if err != nil {
						return fmt.Errorf("error while writing release, error: %w", err)
					}
				}
				return nil
			})
			if err != nil {
				t.Fatalf("error while running the transaction for writing releases to the database, error: %v", err)
			}

			err = dbHandler.WithTransaction(ctx, false, func(ctx context.Context, transaction *sql.Tx) error {
				bracketRow, err := DBSelectBracketHistoryByTimestamp(dbHandler, ctx, transaction, tc.PreparedTimestamp)
				if err != nil {
					return err
				}
				if diff := testutil.CmpDiff(tc.ExpectedBracketRow, bracketRow); diff != "" {
					return fmt.Errorf("releases mismatch (-want +got):\n%s", diff)
				}
				return nil
			})
			if err != nil {
				t.Fatalf("error while running the transaction for writing releases to the database, error: %v", err)
			}
		})
	}
}
