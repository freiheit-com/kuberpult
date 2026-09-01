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

	"github.com/google/go-cmp/cmp"

	"github.com/freiheit-com/kuberpult/pkg/testutil"
	"github.com/freiheit-com/kuberpult/pkg/testutilauth"
	"github.com/freiheit-com/kuberpult/pkg/types"
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
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()
			ctx := testutilauth.MakeTestContext()

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
					return fmt.Errorf("expected no eslVersion, but got %v", *eslVersion)
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

func TestGetCurrentDelays(t *testing.T) {
	tcs := []struct {
		Name                 string
		NumEvents            int
		Cutoff               *EslVersion // nil = no cutoff written yet
		ExpectedDelayEvents  uint64
		ExpectedDelaySeconds float64
	}{
		{
			Name:                 "empty table",
			NumEvents:            0,
			Cutoff:               nil,
			ExpectedDelayEvents:  0,
			ExpectedDelaySeconds: 0,
		},
		{
			Name:                 "single event, no cutoff",
			NumEvents:            1,
			Cutoff:               nil,
			ExpectedDelayEvents:  1,
			ExpectedDelaySeconds: 90,
		},
		{
			Name:                 "no cutoff yet counts everything",
			NumEvents:            3,
			Cutoff:               nil,
			ExpectedDelayEvents:  3,
			ExpectedDelaySeconds: 90,
		},
		{
			Name:                 "exactly one event left",
			NumEvents:            3,
			Cutoff:               types.Ptr(EslVersion(2)),
			ExpectedDelayEvents:  1,
			ExpectedDelaySeconds: 90,
		},
		{
			Name:                 "fully caught up",
			NumEvents:            3,
			Cutoff:               types.Ptr(EslVersion(3)),
			ExpectedDelayEvents:  0,
			ExpectedDelaySeconds: 0,
		},
	}
	for _, tc := range tcs {
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()
			ctx := testutilauth.MakeTestContext()

			dbHandler := setupDB(t)

			err := dbHandler.WithTransaction(ctx, false, func(ctx context.Context, transaction *sql.Tx) error {
				tf := EmptyTransformer{} // we don't care which transformer it is
				for i := 0; i < tc.NumEvents; i++ {
					err := dbHandler.DBWriteEslEventInternal(ctx, "empty", transaction, tf, ESLMetadata{})
					if err != nil {
						return err
					}
				}
				if tc.Cutoff != nil {
					err := DBWriteCutoff(dbHandler, ctx, transaction, *tc.Cutoff)
					if err != nil {
						return err
					}
				}

				// all writes in this transaction share one timestamp, so the age is exact
				created, err := dbHandler.DBReadTransactionTimestamp(ctx, transaction)
				if err != nil {
					return err
				}
				delaySeconds, delayEvents, err := dbHandler.GetCurrentDelays(ctx, transaction, created.Add(90*time.Second))
				if err != nil {
					return err
				}
				if diff := testutil.CmpDiff(tc.ExpectedDelayEvents, delayEvents); diff != "" {
					t.Fatalf("error mismatch (-want, +got):\n%s", diff)
				}
				if diff := testutil.CmpDiff(tc.ExpectedDelaySeconds, delaySeconds); diff != "" {
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
