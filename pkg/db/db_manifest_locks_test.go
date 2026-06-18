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
	"testing"

	"github.com/google/go-cmp/cmp/cmpopts"

	"github.com/freiheit-com/kuberpult/pkg/testutil"
	"github.com/freiheit-com/kuberpult/pkg/types"
)

type lockInput struct {
	App      types.AppName
	Env      types.EnvName
	Metadata LockMetadata
}

func TestDBManifestLocks(t *testing.T) {
	tcs := []struct {
		Name           string
		Locks          []lockInput
		QueryApp       types.AppName
		ExpectedAll    []ManifestLock
		ExpectedForApp []ManifestLock
	}{
		{
			Name: "write one lock - appears in all and for-app queries",
			Locks: []lockInput{
				{App: "app-a", Env: "dev", Metadata: LockMetadata{CreatedByName: "user", CreatedByEmail: "u@example.com", Message: "lock"}},
			},
			QueryApp: "app-a",
			ExpectedAll: []ManifestLock{
				{App: "app-a", Env: "dev", Active: true, EventType: ManifestLockEventTypeCreated,
					Metadata: LockMetadata{CreatedByName: "user", CreatedByEmail: "u@example.com", Message: "lock"}},
			},
			ExpectedForApp: []ManifestLock{
				{App: "app-a", Env: "dev", Active: true, EventType: ManifestLockEventTypeCreated,
					Metadata: LockMetadata{CreatedByName: "user", CreatedByEmail: "u@example.com", Message: "lock"}},
			},
		},
		{
			Name: "locks for different apps - for-app filters correctly",
			Locks: []lockInput{
				{App: "app-a", Env: "dev"},
				{App: "app-b", Env: "dev"},
			},
			QueryApp: "app-a",
			ExpectedAll: []ManifestLock{
				{App: "app-a", Env: "dev", Active: true, EventType: ManifestLockEventTypeCreated},
				{App: "app-b", Env: "dev", Active: true, EventType: ManifestLockEventTypeCreated},
			},
			ExpectedForApp: []ManifestLock{
				{App: "app-a", Env: "dev", Active: true, EventType: ManifestLockEventTypeCreated},
			},
		},
		{
			Name: "two locks for same app on different envs",
			Locks: []lockInput{
				{App: "app-a", Env: "dev"},
				{App: "app-a", Env: "staging"},
			},
			QueryApp: "app-a",
			ExpectedAll: []ManifestLock{
				{App: "app-a", Env: "dev", Active: true, EventType: ManifestLockEventTypeCreated},
				{App: "app-a", Env: "staging", Active: true, EventType: ManifestLockEventTypeCreated},
			},
			ExpectedForApp: []ManifestLock{
				{App: "app-a", Env: "dev", Active: true, EventType: ManifestLockEventTypeCreated},
				{App: "app-a", Env: "staging", Active: true, EventType: ManifestLockEventTypeCreated},
			},
		},
		{
			Name: "no locks for queried app",
			Locks: []lockInput{
				{App: "app-b", Env: "dev"},
			},
			QueryApp: "app-a",
			ExpectedAll: []ManifestLock{
				{App: "app-b", Env: "dev", Active: true, EventType: ManifestLockEventTypeCreated},
			},
			ExpectedForApp: nil,
		},
	}

	for _, tc := range tcs {
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()
			ctx := context.Background()
			dbHandler := setupDB(t)

			err := dbHandler.WithTransaction(ctx, false, func(ctx context.Context, tx *sql.Tx) error {
				for _, l := range tc.Locks {
					//exhaustruct:ignore
					if err := dbHandler.DBWriteManifestLock(ctx, tx, l.App, l.Env, l.Metadata); err != nil {
						return err
					}
				}

				all, err := dbHandler.DBSelectAllActiveManifestLocks(ctx, tx)
				if err != nil {
					return err
				}
				if diff := testutil.CmpDiff(tc.ExpectedAll, all, cmpopts.IgnoreFields(ManifestLock{}, "LockID", "RecordedAt")); diff != "" {
					t.Errorf("all-locks mismatch (-want, +got):\n%s", diff)
				}

				forApp, err := dbHandler.DBSelectAllActiveManifestLocksForApp(ctx, tx, tc.QueryApp)
				if err != nil {
					return err
				}
				if diff := testutil.CmpDiff(tc.ExpectedForApp, forApp, cmpopts.IgnoreFields(ManifestLock{}, "LockID", "RecordedAt"), cmpopts.EquateEmpty()); diff != "" {
					t.Errorf("for-app locks mismatch (-want, +got):\n%s", diff)
				}
				return nil
			})
			if err != nil {
				t.Fatalf("transaction error: %v", err)
			}
		})
	}
}

func TestDBDeleteManifestLock(t *testing.T) {
	tcs := []struct {
		Name      string
		Locks     []lockInput
		DeleteApp types.AppName
		DeleteEnv types.EnvName
		Expected  []ManifestLock
	}{
		{
			Name: "write then delete leaves no active locks",
			Locks: []lockInput{
				{App: "app-a", Env: "dev"},
			},
			DeleteApp: "app-a",
			DeleteEnv: "dev",
			Expected:  nil,
		},
		{
			Name: "delete one of two locks - other remains",
			Locks: []lockInput{
				{App: "app-a", Env: "dev"},
				{App: "app-a", Env: "staging"},
			},
			DeleteApp: "app-a",
			DeleteEnv: "dev",
			Expected: []ManifestLock{
				{App: "app-a", Env: "staging", Active: true, EventType: ManifestLockEventTypeCreated},
			},
		},
	}

	for _, tc := range tcs {
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()
			ctx := context.Background()
			dbHandler := setupDB(t)

			err := dbHandler.WithTransaction(ctx, false, func(ctx context.Context, tx *sql.Tx) error {
				for _, l := range tc.Locks {
					//exhaustruct:ignore
					if err := dbHandler.DBWriteManifestLock(ctx, tx, l.App, l.Env, l.Metadata); err != nil {
						return err
					}
				}
				if err := dbHandler.DBDeleteManifestLock(ctx, tx, tc.DeleteApp, tc.DeleteEnv); err != nil {
					return err
				}
				locks, err := dbHandler.DBSelectAllActiveManifestLocks(ctx, tx)
				if err != nil {
					return err
				}
				if diff := testutil.CmpDiff(tc.Expected, locks, cmpopts.IgnoreFields(ManifestLock{}, "LockID", "RecordedAt"), cmpopts.EquateEmpty()); diff != "" {
					t.Errorf("active locks after delete (-want, +got):\n%s", diff)
				}
				return nil
			})
			if err != nil {
				t.Fatalf("transaction error: %v", err)
			}
		})
	}
}

func TestDBHasActiveManifestLock(t *testing.T) {
	tcs := []struct {
		Name        string
		Locks       []lockInput
		DeleteLocks []lockInput
		QueryApp    types.AppName
		QueryEnv    types.EnvName
		Expected    bool
	}{
		{
			Name:     "no locks - returns false",
			QueryApp: "app-a",
			QueryEnv: "dev",
			Expected: false,
		},
		{
			Name: "active lock for queried app+env - returns true",
			Locks: []lockInput{
				{App: "app-a", Env: "dev"},
			},
			QueryApp: "app-a",
			QueryEnv: "dev",
			Expected: true,
		},
		{
			Name: "lock for different env - returns false",
			Locks: []lockInput{
				{App: "app-a", Env: "staging"},
			},
			QueryApp: "app-a",
			QueryEnv: "dev",
			Expected: false,
		},
		{
			Name: "deleted lock - returns false",
			Locks: []lockInput{
				{App: "app-a", Env: "dev"},
			},
			DeleteLocks: []lockInput{
				{App: "app-a", Env: "dev"},
			},
			QueryApp: "app-a",
			QueryEnv: "dev",
			Expected: false,
		},
	}

	for _, tc := range tcs {
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()
			ctx := context.Background()
			dbHandler := setupDB(t)

			err := dbHandler.WithTransaction(ctx, false, func(ctx context.Context, tx *sql.Tx) error {
				for _, l := range tc.Locks {
					//exhaustruct:ignore
					if err := dbHandler.DBWriteManifestLock(ctx, tx, l.App, l.Env, l.Metadata); err != nil {
						return err
					}
				}
				for _, l := range tc.DeleteLocks {
					if err := dbHandler.DBDeleteManifestLock(ctx, tx, l.App, l.Env); err != nil {
						return err
					}
				}
				got, err := dbHandler.DBHasActiveManifestLock(ctx, tx, tc.QueryApp, tc.QueryEnv)
				if err != nil {
					return err
				}
				if diff := testutil.CmpDiff(tc.Expected, got); diff != "" {
					t.Errorf("has-active-lock mismatch (-want, +got):\n%s", diff)
				}
				return nil
			})
			if err != nil {
				t.Fatalf("transaction error: %v", err)
			}
		})
	}
}
