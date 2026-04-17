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
	timeFirst := calcTime(1)
	timeSecond := calcTime(2)
	tcs := []struct {
		Name                   string
		PreparedBracketRows    []BracketRow
		TransformerIndexToTest TransformerID
		ExpectedBracketRow     *BracketRow
		ExpectedErr            error
	}{
		{
			Name:                   "no data",
			PreparedBracketRows:    []BracketRow{},
			TransformerIndexToTest: 1,
			ExpectedBracketRow:     nil,
		},
		{
			Name: "just one result",
			PreparedBracketRows: []BracketRow{
				{
					CreatedAt: timeFirst,
					AllBracketsJsonBlob: BracketJsonBlob{
						BracketMap: map[types.ArgoBracketName]AppNames{
							"b1": {"app1", "app2"},
						},
					},
				},
			},
			TransformerIndexToTest: 1,
			ExpectedBracketRow: &BracketRow{
				CreatedAt: timeFirst,
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
					CreatedAt: timeFirst,
					AllBracketsJsonBlob: BracketJsonBlob{
						BracketMap: map[types.ArgoBracketName]AppNames{
							"b1": {"app1", "app2"},
						},
					},
				},
				{
					CreatedAt: timeSecond,
					AllBracketsJsonBlob: BracketJsonBlob{
						BracketMap: map[types.ArgoBracketName]AppNames{
							"b1": {"app3", "app2"},
						},
					},
				},
			},
			TransformerIndexToTest: 2,
			ExpectedBracketRow: &BracketRow{
				CreatedAt: timeSecond,
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
					CreatedAt: timeFirst,
					AllBracketsJsonBlob: BracketJsonBlob{
						BracketMap: map[types.ArgoBracketName]AppNames{
							"b1": {"app1", "app2"},
						},
					},
				},
				{
					CreatedAt: timeSecond,
					AllBracketsJsonBlob: BracketJsonBlob{
						BracketMap: map[types.ArgoBracketName]AppNames{
							"b1": {"app3", "app2"},
						},
					},
				},
			},
			TransformerIndexToTest: 1, // This means we look back into history
			ExpectedBracketRow: &BracketRow{
				CreatedAt: timeFirst,
				AllBracketsJsonBlob: BracketJsonBlob{
					BracketMap: map[types.ArgoBracketName]AppNames{
						"b1": {"app1", "app2"},
					},
				},
			},
		},
	}

	for _, tc := range tcs {
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()
			ctx := testutilauth.MakeTestContext()
			dbHandler := setupDB(t)
			err := dbHandler.WithTransaction(ctx, false, func(ctx context.Context, transaction *sql.Tx) error {
				for range 10 {
					err := dbHandler.DBWriteEslEventInternal(ctx, "empty", transaction, interface{}(nil), ESLMetadata{})
					if err != nil {
						return fmt.Errorf("error while writing release, error: %w", err)
					}
				}
				return nil
			})
			if err != nil {
				t.Fatal(err)
			}

			for index, bracketRow := range tc.PreparedBracketRows {
				err := dbHandler.WithTransaction(ctx, false, func(ctx context.Context, transaction *sql.Tx) error {
					err := DBInsertBracketHistory(ctx, dbHandler, transaction, bracketRow, TransformerID(1+index))
					if err != nil {
						return fmt.Errorf("error while writing release, error: %w", err)
					}
					return nil
				})
				if err != nil {
					t.Fatalf("error while running the transaction for writing releases to the database, error: %v", err)
				}
			}

			err = dbHandler.WithTransaction(ctx, true, func(ctx context.Context, transaction *sql.Tx) error {
				bracketRow, err := DBSelectBracketHistoryById(ctx, dbHandler, transaction, tc.TransformerIndexToTest)
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
	timeFirst := calcTime(1)
	timeSecond := calcTime(2)
	timeThird := calcTime(3)
	type AppBracketTime struct {
		App     types.AppName
		Bracket types.ArgoBracketName
		Time    time.Time
	}
	tcs := []struct {
		Name string
		// Given
		AddAppBrackets    []AppBracketTime
		DeleteAppBrackets []AppBracketTime
		// When
		TransformerIndexToTest TransformerID
		// Then
		ExpectedBracketRow *BracketRow
		ExpectedErr        error
	}{
		{
			Name:                   "no data",
			AddAppBrackets:         []AppBracketTime{},
			DeleteAppBrackets:      []AppBracketTime{},
			TransformerIndexToTest: 1,
			ExpectedBracketRow:     nil,
		},
		{
			Name: "one entry",
			AddAppBrackets: []AppBracketTime{
				{
					App:     types.AppName("app1"),
					Bracket: types.ArgoBracketName("b1"),
					Time:    timeFirst,
				},
			},
			DeleteAppBrackets:      []AppBracketTime{},
			TransformerIndexToTest: 10,
			ExpectedBracketRow: &BracketRow{
				CreatedAt: timeFirst,
				AllBracketsJsonBlob: BracketJsonBlob{
					BracketMap: map[types.ArgoBracketName]AppNames{
						"b1": {"app1"},
					},
				},
			},
		},
		{
			Name: "three entries on two buckets are sorted",
			AddAppBrackets: []AppBracketTime{
				{
					App:     types.AppName("app3"),
					Bracket: types.ArgoBracketName("b1"),
					Time:    timeFirst,
				},
				{
					App:     types.AppName("app2"),
					Bracket: types.ArgoBracketName("b2"),
					Time:    timeSecond,
				},
				{
					App:     types.AppName("app1"),
					Bracket: types.ArgoBracketName("b1"),
					Time:    timeThird,
				},
			},
			DeleteAppBrackets:      []AppBracketTime{},
			TransformerIndexToTest: 12,
			ExpectedBracketRow: &BracketRow{
				CreatedAt: timeFirst,
				AllBracketsJsonBlob: BracketJsonBlob{
					BracketMap: map[types.ArgoBracketName]AppNames{
						"b1": {"app1", "app3"},
						"b2": {"app2"},
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
					Time:    timeFirst,
				},
			},
			DeleteAppBrackets: []AppBracketTime{
				{
					App:     types.AppName("app1"),
					Bracket: types.ArgoBracketName("b1"),
					Time:    timeSecond,
				},
			},
			TransformerIndexToTest: 20,
			ExpectedBracketRow: &BracketRow{
				CreatedAt: timeSecond,
				AllBracketsJsonBlob: BracketJsonBlob{
					BracketMap: map[types.ArgoBracketName]AppNames{},
				},
			},
		},
		{
			Name: "add one entry without bracket, delete same entry",
			AddAppBrackets: []AppBracketTime{
				{
					App:     types.AppName("app1"),
					Bracket: types.ArgoBracketName(""), // this will be stored in the DB as bracket "app1"
					Time:    timeFirst,
				},
			},
			DeleteAppBrackets: []AppBracketTime{
				{
					App:     types.AppName("app1"),
					Bracket: types.ArgoBracketName("app1"), // therefore this will be called with the bracketname "app1"
					Time:    timeSecond,
				},
			},
			TransformerIndexToTest: 20,
			ExpectedBracketRow: &BracketRow{
				CreatedAt: timeSecond,
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
					Time:    timeFirst,
				},
				{
					App:     types.AppName("app2"),
					Bracket: types.ArgoBracketName("b1"),
					Time:    timeSecond,
				},
			},
			DeleteAppBrackets: []AppBracketTime{
				{
					App:     types.AppName("app1"),
					Bracket: types.ArgoBracketName("b1"),
					Time:    timeThird,
				},
			},
			TransformerIndexToTest: 20,
			ExpectedBracketRow: &BracketRow{
				CreatedAt: timeThird,
				AllBracketsJsonBlob: BracketJsonBlob{
					BracketMap: map[types.ArgoBracketName]AppNames{
						"b1": {"app2"},
					},
				},
			},
		},
		{
			Name: "add one entry, delete non-existent entry",
			AddAppBrackets: []AppBracketTime{
				{
					App:     types.AppName("app1"),
					Bracket: types.ArgoBracketName("b1"),
					Time:    timeFirst,
				},
			},
			DeleteAppBrackets: []AppBracketTime{
				{
					App:     types.AppName("appDoesNotExist"),
					Bracket: types.ArgoBracketName("b1"),
					Time:    timeSecond,
				},
			},
			TransformerIndexToTest: 20,
			ExpectedBracketRow: &BracketRow{
				CreatedAt: timeFirst,
				AllBracketsJsonBlob: BracketJsonBlob{
					BracketMap: map[types.ArgoBracketName]AppNames{
						"b1": {"app1"},
					},
				},
			},
		},
		{
			Name: "add one entry, delete it by overwriting with ''",
			AddAppBrackets: []AppBracketTime{
				{
					App:     types.AppName("app1"),
					Bracket: types.ArgoBracketName("b1"),
					Time:    timeFirst,
				},
				{
					App:     types.AppName("app1"),
					Bracket: types.ArgoBracketName(""),
					Time:    timeFirst,
				},
			},
			DeleteAppBrackets:      []AppBracketTime{},
			TransformerIndexToTest: 11,
			ExpectedBracketRow: &BracketRow{
				CreatedAt: timeFirst,
				AllBracketsJsonBlob: BracketJsonBlob{
					BracketMap: map[types.ArgoBracketName]AppNames{
						"app1": {"app1"},
					},
				},
			},
		},
		{
			Name: "add two brackets, then update one of them",
			AddAppBrackets: []AppBracketTime{
				{
					App:     types.AppName("app1"),
					Bracket: types.ArgoBracketName("b1"),
					Time:    timeFirst,
				},
				{
					App:     types.AppName("app2"),
					Bracket: types.ArgoBracketName("foo"),
					Time:    timeFirst,
				},
				{
					App:     types.AppName("app1"),
					Bracket: types.ArgoBracketName("b2"),
					Time:    timeFirst,
				},
				{
					App:     types.AppName("app1"),
					Bracket: types.ArgoBracketName("b3"),
					Time:    timeFirst,
				},
			},
			DeleteAppBrackets:      []AppBracketTime{},
			TransformerIndexToTest: 13,
			ExpectedBracketRow: &BracketRow{
				CreatedAt: timeFirst,
				AllBracketsJsonBlob: BracketJsonBlob{
					BracketMap: map[types.ArgoBracketName]AppNames{
						"b3":  {"app1"},
						"foo": {"app2"},
					},
				},
			},
		},
	}

	for _, tc := range tcs {
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()
			ctx := testutilauth.MakeTestContext()
			dbHandler := setupDB(t)

			err := dbHandler.WithTransaction(ctx, false, func(ctx context.Context, transaction *sql.Tx) error {
				for range 30 {
					err := dbHandler.DBWriteEslEventInternal(ctx, "empty", transaction, interface{}(nil), ESLMetadata{})
					if err != nil {
						return fmt.Errorf("error while writing release, error: %w", err)
					}
				}
				return nil
			})
			if err != nil {
				t.Fatal(err)
			}

			for index, appBracket := range tc.AddAppBrackets {
				err := dbHandler.WithTransaction(ctx, false, func(ctx context.Context, transaction *sql.Tx) error {
					_, err := HandleBracketsUpdate(ctx, dbHandler, transaction, appBracket.App, appBracket.Bracket, appBracket.Time, TransformerID(10+index))
					if err != nil {
						return fmt.Errorf("error while writing release, error: %w", err)
					}
					return nil
				})
				if err != nil {
					t.Fatalf("error while running the transaction for writing releases to the database, error: %v", err)
				}
			}
			for index, appBracket := range tc.DeleteAppBrackets {
				err := dbHandler.WithTransaction(ctx, false, func(ctx context.Context, transaction *sql.Tx) error {
					err := HandleDeleteAppFromBracket(ctx, dbHandler, transaction, appBracket.App, appBracket.Bracket, appBracket.Time, TransformerID(20+index))
					if err != nil {
						return fmt.Errorf("error while writing release, error: %w", err)
					}
					return nil
				})
				if err != nil {
					t.Fatalf("error while running the transaction for writing releases to the database, error: %v", err)
				}
			}

			err = dbHandler.WithTransaction(ctx, true, func(ctx context.Context, transaction *sql.Tx) error {
				bracketRow, err := DBSelectBracketHistoryById(ctx, dbHandler, transaction, tc.TransformerIndexToTest)
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

// TestHandleBracketDoubleDeletion is about the case where 2 apps are deleted at the same time - within one transaction
func TestHandleBracketDoubleDeletion(t *testing.T) {
	calcTime := func(sec int) time.Time { return time.Date(2000, 1, 1, 0, 0, sec, 0, time.UTC) }
	timeFirst := calcTime(1)
	timeSecond := calcTime(2)
	timeThird := calcTime(3)
	type AppBracketTime struct {
		App     types.AppName
		Bracket types.ArgoBracketName
		Time    time.Time
	}
	tcs := []struct {
		Name string
		// Given
		Setup                             []AppBracketTime
		DeleteAppBracketsOnly1Transaction []AppBracketTime // multiple deletions within just 1 transaction
		// When
		TransformerIndexToTest TransformerID
		// Then
		ExpectedBracketRow *BracketRow
		ExpectedErr        error
	}{
		{
			Name: "one entry",
			Setup: []AppBracketTime{
				{
					App:     types.AppName("app1"),
					Bracket: types.ArgoBracketName("b1"),
					Time:    timeFirst,
				},
				{
					App:     types.AppName("app2"),
					Bracket: types.ArgoBracketName("b1"),
					Time:    timeSecond,
				},
			},
			DeleteAppBracketsOnly1Transaction: []AppBracketTime{
				{
					App:     types.AppName("app1"),
					Bracket: types.ArgoBracketName("b1"),
					Time:    timeThird,
				},
				{
					App:     types.AppName("app2"),
					Bracket: types.ArgoBracketName("b1"),
					Time:    timeThird, // same as above!
				},
			},
			TransformerIndexToTest: 21,
			ExpectedBracketRow: &BracketRow{
				CreatedAt: timeThird,
				AllBracketsJsonBlob: BracketJsonBlob{
					BracketMap: map[types.ArgoBracketName]AppNames{},
				},
			},
		},
	}

	for _, tc := range tcs {
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()
			ctx := testutilauth.MakeTestContext()
			dbHandler := setupDB(t)

			err := dbHandler.WithTransaction(ctx, false, func(ctx context.Context, transaction *sql.Tx) error {
				for range 30 {
					err := dbHandler.DBWriteEslEventInternal(ctx, "empty", transaction, interface{}(nil), ESLMetadata{})
					if err != nil {
						return fmt.Errorf("error while writing release, error: %w", err)
					}
				}
				return nil
			})
			if err != nil {
				t.Fatal(err)
			}

			for index, appBracket := range tc.Setup {
				err := dbHandler.WithTransaction(ctx, false, func(ctx context.Context, transaction *sql.Tx) error {
					_, err := HandleBracketsUpdate(ctx, dbHandler, transaction, appBracket.App, appBracket.Bracket, appBracket.Time, TransformerID(10+index))
					if err != nil {
						return fmt.Errorf("error while writing release, error: %w", err)
					}
					return nil
				})
				if err != nil {
					t.Fatalf("error while running the transaction for writing releases to the database, error: %v", err)
				}
			}
			err = dbHandler.WithTransaction(ctx, false, func(ctx context.Context, transaction *sql.Tx) error {
				for index, appBracket := range tc.DeleteAppBracketsOnly1Transaction {
					t.Logf("app-bracket deletion ongoing: %v", appBracket)
					err := HandleDeleteAppFromBracket(ctx, dbHandler, transaction, appBracket.App, appBracket.Bracket, appBracket.Time, TransformerID(20+index))
					if err != nil {
						return fmt.Errorf("error while writing release, error: %w", err)
					}
				}
				return nil
			})
			if err != nil {
				t.Fatalf("error while running the transaction for writing releases to the database, error: %v", err)
			}

			err = dbHandler.WithTransaction(ctx, true, func(ctx context.Context, transaction *sql.Tx) error {
				bracketRow, err := DBSelectBracketHistoryById(ctx, dbHandler, transaction, tc.TransformerIndexToTest)
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
