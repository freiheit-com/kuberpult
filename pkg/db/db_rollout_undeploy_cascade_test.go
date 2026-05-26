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

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"

	"github.com/freiheit-com/kuberpult/pkg/testutilauth"
	"github.com/freiheit-com/kuberpult/pkg/types"
)

func TestDBRolloutUndeployCascade(t *testing.T) {
	tcs := []struct {
		Name          string
		UpsertRows    []*RolloutShouldUndeployCascade
		IncrementRows []*RolloutShouldUndeployCascade // rows to increment attempts on
		DeleteRows    []*RolloutShouldUndeployCascade // rows to delete after upsert
		BatchLimit    int
		ExpectedRows  []*RolloutShouldUndeployCascade
	}{
		{
			Name: "upsert and read one row",
			UpsertRows: []*RolloutShouldUndeployCascade{
				{ArgoApp: "staging-my-app", Env: "staging"},
			},
			BatchLimit: 10,
			ExpectedRows: []*RolloutShouldUndeployCascade{
				{ArgoApp: "staging-my-app", Env: "staging", Attempts: 0},
			},
		},
		{
			Name: "upsert two rows for different (argo_app, env) pairs",
			UpsertRows: []*RolloutShouldUndeployCascade{
				{ArgoApp: "staging-app1", Env: "staging"},
				{ArgoApp: "development-app1", Env: "development"},
			},
			BatchLimit: 10,
			ExpectedRows: []*RolloutShouldUndeployCascade{
				{ArgoApp: "staging-app1", Env: "staging"},
				{ArgoApp: "development-app1", Env: "development"},
			},
		},
		{
			Name: "upsert is idempotent — re-inserting the same (argo_app, env) does not reset attempts",
			UpsertRows: []*RolloutShouldUndeployCascade{
				{ArgoApp: "staging-app1", Env: "staging"},
				{ArgoApp: "staging-app1", Env: "staging"}, // duplicate
			},
			IncrementRows: []*RolloutShouldUndeployCascade{
				{ArgoApp: "staging-app1", Env: "staging"},
				{ArgoApp: "staging-app1", Env: "staging"}, // attempts = 2
			},
			BatchLimit: 10,
			ExpectedRows: []*RolloutShouldUndeployCascade{
				{ArgoApp: "staging-app1", Env: "staging", Attempts: 2},
			},
		},
		{
			Name: "delete removes the row",
			UpsertRows: []*RolloutShouldUndeployCascade{
				{ArgoApp: "staging-app1", Env: "staging"},
				{ArgoApp: "staging-app2", Env: "staging"},
			},
			DeleteRows: []*RolloutShouldUndeployCascade{
				{ArgoApp: "staging-app1", Env: "staging"},
			},
			BatchLimit: 10,
			ExpectedRows: []*RolloutShouldUndeployCascade{
				{ArgoApp: "staging-app2", Env: "staging"},
			},
		},
		{
			Name: "batch limit caps the number of returned rows",
			UpsertRows: []*RolloutShouldUndeployCascade{
				{ArgoApp: "staging-app1", Env: "staging"},
				{ArgoApp: "staging-app2", Env: "staging"},
				{ArgoApp: "staging-app3", Env: "staging"},
			},
			BatchLimit: 2,
			ExpectedRows: []*RolloutShouldUndeployCascade{
				{ArgoApp: "staging-app1", Env: "staging"},
				{ArgoApp: "staging-app2", Env: "staging"},
			},
		},
	}
	for _, tc := range tcs {
		t.Run(tc.Name, func(t *testing.T) {
			ctx := testutilauth.MakeTestContext()
			dbHandler := setupDB(t)

			errW := dbHandler.WithTransaction(ctx, false, func(ctx context.Context, transaction *sql.Tx) error {
				for _, row := range tc.UpsertRows {
					if err := dbHandler.UpsertRolloutUndeployCascade(ctx, transaction, row.ArgoApp, row.Env, row.NotBeforeTransformerEslId); err != nil {
						return err
					}
				}
				for _, row := range tc.IncrementRows {
					if err := dbHandler.DBIncrementRolloutUndeployCascadeAttempts(ctx, transaction, row.ArgoApp, row.Env); err != nil {
						return err
					}
				}
				for _, row := range tc.DeleteRows {
					if err := dbHandler.DBDeleteRolloutUndeployCascade(ctx, transaction, row.ArgoApp, row.Env); err != nil {
						return err
					}
				}
				return nil
			})
			if errW != nil {
				t.Fatalf("write transaction error: %v", errW)
			}

			errR := dbHandler.WithTransaction(ctx, true, func(ctx context.Context, transaction *sql.Tx) error {
				got, err := dbHandler.DBReadRolloutUndeployCascadeBatch(ctx, transaction, tc.BatchLimit)
				if err != nil {
					return err
				}
				// Created is set by NOW() on the server and is irrelevant to these assertions.
				if diff := cmp.Diff(tc.ExpectedRows, got,
					cmpopts.IgnoreFields(RolloutShouldUndeployCascade{}, "Created"),
					cmpopts.SortSlices(func(a, b *RolloutShouldUndeployCascade) bool {
						if a.ArgoApp != b.ArgoApp {
							return a.ArgoApp < b.ArgoApp
						}
						return a.Env < b.Env
					}),
				); diff != "" {
					t.Errorf("rows mismatch (-want, +got):\n%s", diff)
				}
				return nil
			})
			if errR != nil {
				t.Fatalf("read transaction error: %v", errR)
			}
		})
	}
}

// Compile-time guard: the consumer signature is stable so the rollout-service can rely on it.
var _ = func(h *DBHandler, ctx context.Context, tx *sql.Tx, argoApp string, env types.EnvName, eslId TransformerID) error {
	if _, err := h.DBReadRolloutUndeployCascadeBatch(ctx, tx, 1); err != nil {
		return err
	}
	if err := h.UpsertRolloutUndeployCascade(ctx, tx, argoApp, env, eslId); err != nil {
		return err
	}
	if err := h.DBIncrementRolloutUndeployCascadeAttempts(ctx, tx, argoApp, env); err != nil {
		return err
	}
	return h.DBDeleteRolloutUndeployCascade(ctx, tx, argoApp, env)
}
