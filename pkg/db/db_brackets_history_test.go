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
	calcTime := func(sec int) time.Time { return time.Date(2000, 1, 1, 0, 0, sec, 0, time.UTC) }
	timeA := calcTime(1)
	timeB := calcTime(2)
	//timeC := calcTime(3)
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
			PreparedTimestamp:   timeA,
			ExpectedBracketRow:  nil,
		},
		{
			Name: "just one result",
			PreparedBracketRows: []BracketRow{
				{
					CreatedAt: timeA,
					AllBracketsJsonBlob: BracketJsonBlob{
						BracketMap: map[types.ArgoBracketName]AppNames{
							"b1": {"app1", "app2"},
						},
					},
				},
			},
			PreparedTimestamp: timeA,
			ExpectedBracketRow: &BracketRow{
				CreatedAt: timeA,
				AllBracketsJsonBlob: BracketJsonBlob{
					BracketMap: map[types.ArgoBracketName]AppNames{
						"b1": {"app1", "app2"},
					},
				},
			},
		},
		{
			Name: "two inputs, second wins",
			PreparedBracketRows: []BracketRow{
				{
					CreatedAt: timeA,
					AllBracketsJsonBlob: BracketJsonBlob{
						BracketMap: map[types.ArgoBracketName]AppNames{
							"b1": {"app1", "app2"},
						},
					},
				},
				{
					CreatedAt: timeB,
					AllBracketsJsonBlob: BracketJsonBlob{
						BracketMap: map[types.ArgoBracketName]AppNames{
							"b1": {"app3", "app2"},
						},
					},
				},
			},
			PreparedTimestamp: timeB,
			ExpectedBracketRow: &BracketRow{
				CreatedAt: timeB,
				AllBracketsJsonBlob: BracketJsonBlob{
					BracketMap: map[types.ArgoBracketName]AppNames{
						"b1": {"app3", "app2"},
					},
				},
			},
		},
		{
			Name: "two inputs, first one wins because, we're looking back into history",
			PreparedBracketRows: []BracketRow{
				{
					CreatedAt: timeA,
					AllBracketsJsonBlob: BracketJsonBlob{
						BracketMap: map[types.ArgoBracketName]AppNames{
							"b1": {"app1", "app2"},
						},
					},
				},
				{
					CreatedAt: timeB,
					AllBracketsJsonBlob: BracketJsonBlob{
						BracketMap: map[types.ArgoBracketName]AppNames{
							"b1": {"app3", "app2"},
						},
					},
				},
			},
			PreparedTimestamp: timeA, // This means we look back into history
			ExpectedBracketRow: &BracketRow{
				CreatedAt: timeA,
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

			for _, bracketRow := range tc.PreparedBracketRows {
				err := dbHandler.WithTransaction(ctx, false, func(ctx context.Context, transaction *sql.Tx) error {
					err := DBInsertBracketHistory(dbHandler, ctx, transaction, bracketRow)
					if err != nil {
						return fmt.Errorf("error while writing release, error: %w", err)
					}
					return nil
				})
				if err != nil {
					t.Fatalf("error while running the transaction for writing releases to the database, error: %v", err)
				}
			}

			err := dbHandler.WithTransaction(ctx, true, func(ctx context.Context, transaction *sql.Tx) error {
				bracketRow, err := DBSelectBracketHistoryByTimestamp(dbHandler, ctx, transaction, &tc.PreparedTimestamp)
				if err != nil {
					return err
				}
				testutil.DiffOrFail(t, "bracketRow", tc.ExpectedBracketRow, bracketRow)
				return nil
			})
			if err != nil {
				t.Fatalf("error while running the transaction for writing releases to the database, error: %v", err)
			}
		})
	}
}

func TestHandleBracketUpdates(t *testing.T) {
	calcTime := func(sec int) time.Time { return time.Date(2000, 1, 1, 0, 0, sec, 0, time.UTC) }
	timeA := calcTime(1)
	timeB := calcTime(2)
	timeC := calcTime(3)
	type AppBracketTime struct {
		App     types.AppName
		Bracket types.ArgoBracketName
		Time    time.Time
	}
	tcs := []struct {
		Name               string
		AddAppBrackets     []AppBracketTime
		DeleteAppBrackets  []AppBracketTime
		PreparedTimestamp  time.Time
		ExpectedBracketRow *BracketRow
		ExpectedErr        error
	}{
		{
			Name:               "no data",
			AddAppBrackets:     []AppBracketTime{},
			DeleteAppBrackets:  []AppBracketTime{},
			PreparedTimestamp:  timeA,
			ExpectedBracketRow: nil,
		},
		{
			Name: "one entry",
			AddAppBrackets: []AppBracketTime{
				{
					App:     types.AppName("app1"),
					Bracket: types.ArgoBracketName("b1"),
					Time:    timeA,
				},
			},
			DeleteAppBrackets: []AppBracketTime{},
			PreparedTimestamp: timeA,
			ExpectedBracketRow: &BracketRow{
				CreatedAt: timeA,
				AllBracketsJsonBlob: BracketJsonBlob{
					BracketMap: map[types.ArgoBracketName]AppNames{
						"b1": {"app1"},
					},
				},
			},
		},
		{
			Name: "add one entry, delete same entry",
			AddAppBrackets: []AppBracketTime{
				{
					App:     types.AppName("app1"),
					Bracket: types.ArgoBracketName("b1"),
					Time:    timeA,
				},
			},
			DeleteAppBrackets: []AppBracketTime{
				{
					App:     types.AppName("app1"),
					Bracket: types.ArgoBracketName("b1"),
					Time:    timeB,
				},
			},
			PreparedTimestamp: timeB,
			ExpectedBracketRow: &BracketRow{
				CreatedAt: timeB,
				AllBracketsJsonBlob: BracketJsonBlob{
					BracketMap: map[types.ArgoBracketName]AppNames{},
				},
			},
		},
		{
			Name: "add two entries, delete first entry",
			AddAppBrackets: []AppBracketTime{
				{
					App:     types.AppName("app1"),
					Bracket: types.ArgoBracketName("b1"),
					Time:    timeA,
				},
				{
					App:     types.AppName("app2"),
					Bracket: types.ArgoBracketName("b1"),
					Time:    timeB,
				},
			},
			DeleteAppBrackets: []AppBracketTime{
				{
					App:     types.AppName("app1"),
					Bracket: types.ArgoBracketName("b1"),
					Time:    timeC,
				},
			},
			PreparedTimestamp: timeC,
			ExpectedBracketRow: &BracketRow{
				CreatedAt: timeC,
				AllBracketsJsonBlob: BracketJsonBlob{
					BracketMap: map[types.ArgoBracketName]AppNames{},
				},
			},
		},
		{
			Name: "add one entry, delete non-existent entry",
			AddAppBrackets: []AppBracketTime{
				{
					App:     types.AppName("app1"),
					Bracket: types.ArgoBracketName("b1"),
					Time:    timeA,
				},
			},
			DeleteAppBrackets: []AppBracketTime{
				{
					App:     types.AppName("appDoesNotExist"),
					Bracket: types.ArgoBracketName("b1"),
					Time:    timeB,
				},
			},
			PreparedTimestamp: timeB,
			ExpectedBracketRow: &BracketRow{
				CreatedAt: timeA,
				AllBracketsJsonBlob: BracketJsonBlob{
					BracketMap: map[types.ArgoBracketName]AppNames{
						"b1": {"app1"},
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

			for _, appBracket := range tc.AddAppBrackets {
				err := dbHandler.WithTransaction(ctx, false, func(ctx context.Context, transaction *sql.Tx) error {
					err := HandleBracketsUpdate(dbHandler, ctx, transaction, appBracket.App, appBracket.Bracket, appBracket.Time)
					if err != nil {
						return fmt.Errorf("error while writing release, error: %w", err)
					}
					return nil
				})
				if err != nil {
					t.Fatalf("error while running the transaction for writing releases to the database, error: %v", err)
				}
			}
			for _, appBracket := range tc.DeleteAppBrackets {
				err := dbHandler.WithTransaction(ctx, false, func(ctx context.Context, transaction *sql.Tx) error {
					err := HandleBracketsDeletion(dbHandler, ctx, transaction, appBracket.App, appBracket.Bracket, appBracket.Time)
					if err != nil {
						return fmt.Errorf("error while writing release, error: %w", err)
					}
					return nil
				})
				if err != nil {
					t.Fatalf("error while running the transaction for writing releases to the database, error: %v", err)
				}
			}

			err := dbHandler.WithTransaction(ctx, true, func(ctx context.Context, transaction *sql.Tx) error {
				bracketRow, err := DBSelectBracketHistoryByTimestamp(dbHandler, ctx, transaction, &tc.PreparedTimestamp)
				if err != nil {
					return err
				}
				testutil.DiffOrFail(t, "bracketRow", tc.ExpectedBracketRow, bracketRow)
				return nil
			})
			if err != nil {
				t.Fatalf("error while running the transaction for writing releases to the database, error: %v", err)
			}
		})
	}
}
