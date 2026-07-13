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

package undeploy

import (
	"context"
	"database/sql"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/argoproj/argo-cd/v2/pkg/apiclient/application"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/freiheit-com/kuberpult/pkg/db"
	"github.com/freiheit-com/kuberpult/pkg/types"
)

// mockAppDeleter records Delete calls and returns a programmed error.
type mockAppDeleter struct {
	deleteCalls []application.ApplicationDeleteRequest
	deleteErr   error
}

func (m *mockAppDeleter) Delete(ctx context.Context, in *application.ApplicationDeleteRequest, opts ...grpc.CallOption) (*application.ApplicationResponse, error) {
	m.deleteCalls = append(m.deleteCalls, *in)
	return nil, m.deleteErr
}

func TestProcessRow(t *testing.T) {
	tcs := []struct {
		Name              string
		SeedAttempts      int   // attempts value to insert with the row before processRow runs
		DeleteErr         error // what the Argo client returns
		WantArgoAppName   string
		WantArgoCascade   bool
		WantRowGone       bool // after processRow, the row should be deleted
		WantAttemptsAfter int  // attempts column value if the row is still present
	}{
		{
			Name:            "happy path: Argo Delete succeeds, row removed",
			SeedAttempts:    0,
			DeleteErr:       nil,
			WantArgoAppName: "staging-my-app",
			WantArgoCascade: true,
			WantRowGone:     true,
		},
		{
			Name:            "NotFound treated as idempotent success, row removed",
			SeedAttempts:    1,
			DeleteErr:       status.Error(codes.NotFound, "no such app"),
			WantArgoAppName: "staging-my-app",
			WantArgoCascade: true,
			WantRowGone:     true,
		},
		{
			Name:              "transient error: row kept, attempts incremented",
			SeedAttempts:      0,
			DeleteErr:         status.Error(codes.Unavailable, "argo down"),
			WantArgoAppName:   "staging-my-app",
			WantArgoCascade:   true,
			WantRowGone:       false,
			WantAttemptsAfter: 1,
		},
		{
			Name:            "attempts budget exhausted: row removed",
			SeedAttempts:    maxAttempts - 1,
			DeleteErr:       status.Error(codes.Unavailable, "argo down"),
			WantArgoAppName: "staging-my-app",
			WantArgoCascade: true,
			WantRowGone:     true,
		},
		{
			Name:              "non-grpc error treated as transient",
			SeedAttempts:      0,
			DeleteErr:         errors.New("network blip"),
			WantArgoAppName:   "staging-my-app",
			WantArgoCascade:   true,
			WantRowGone:       false,
			WantAttemptsAfter: 1,
		},
	}
	for _, tc := range tcs {
		t.Run(tc.Name, func(t *testing.T) {
			ctx := context.Background()
			dbHandler := setupDB(t)
			const argoApp = "my-app"
			const envName types.EnvName = "staging"

			// Seed the table with one row at the desired attempts level.
			errW := dbHandler.WithTransaction(ctx, false, func(ctx context.Context, tx *sql.Tx) error {
				if err := dbHandler.UpsertRolloutUndeployCascade(ctx, tx, argoApp, envName, false, 0); err != nil {
					return err
				}
				for i := 0; i < tc.SeedAttempts; i++ {
					if err := dbHandler.DBIncrementRolloutUndeployCascadeAttempts(ctx, tx, argoApp, envName); err != nil {
						return err
					}
				}
				return nil
			})
			if errW != nil {
				t.Fatalf("seed: %v", errW)
			}

			mock := &mockAppDeleter{deleteCalls: nil, deleteErr: tc.DeleteErr}
			row := &db.RolloutShouldUndeployCascade{ArgoApp: argoApp, Env: envName, Attempts: tc.SeedAttempts}
			processRow(ctx, dbHandler, mock, row)

			// Argo Delete invariants: always exactly one call with the right name & cascade.
			if len(mock.deleteCalls) != 1 {
				t.Fatalf("expected exactly one Argo Delete call, got %d", len(mock.deleteCalls))
			}
			call := mock.deleteCalls[0]
			if call.Name == nil || *call.Name != tc.WantArgoAppName {
				gotName := "<nil>"
				if call.Name != nil {
					gotName = *call.Name
				}
				t.Errorf("Argo Delete name: want %q, got %q", tc.WantArgoAppName, gotName)
			}
			if call.Cascade == nil || *call.Cascade != tc.WantArgoCascade {
				t.Errorf("Argo Delete cascade: want %v, got %v", tc.WantArgoCascade, call.Cascade)
			}

			// DB invariants: row present or absent, and attempts value if present.
			var rowsAfter []*db.RolloutShouldUndeployCascade
			errR := dbHandler.WithTransaction(ctx, true, func(ctx context.Context, tx *sql.Tx) error {
				got, err := dbHandler.DBReadRolloutUndeployCascadeBatch(ctx, tx, 10)
				if err != nil {
					return err
				}
				rowsAfter = got
				return nil
			})
			if errR != nil {
				t.Fatalf("read after: %v", errR)
			}
			if tc.WantRowGone {
				if len(rowsAfter) != 0 {
					t.Errorf("expected row removed, but got %d rows: %+v", len(rowsAfter), rowsAfter)
				}
			} else {
				if len(rowsAfter) != 1 {
					t.Fatalf("expected row to remain (1), got %d", len(rowsAfter))
				}
				if rowsAfter[0].Attempts != tc.WantAttemptsAfter {
					t.Errorf("attempts after processRow: want %d, got %d", tc.WantAttemptsAfter, rowsAfter[0].Attempts)
				}
			}
		})
	}
}

func TestProcessOneBatch(t *testing.T) {
	tcs := []struct {
		Name          string
		SeedRows      int   // number of (app, env) rows to seed before the batch runs
		DeleteErr     error // Argo Delete result for all rows
		WantProcessed int   // expected return value of processOneBatch
		WantArgoCalls int   // number of Argo Delete calls (== rows processed)
		WantRowsAfter int   // rows left in the table after one batch
	}{
		{
			Name:          "empty queue: no Argo calls, processed=0",
			SeedRows:      0,
			DeleteErr:     nil,
			WantProcessed: 0,
			WantArgoCalls: 0,
			WantRowsAfter: 0,
		},
		{
			Name:          "small batch (3 rows) drained in one pass",
			SeedRows:      3,
			DeleteErr:     nil,
			WantProcessed: 3,
			WantArgoCalls: 3,
			WantRowsAfter: 0,
		},
		{
			Name:          "transient error: rows kept, attempts bumped",
			SeedRows:      3,
			DeleteErr:     status.Error(codes.Unavailable, "argo down"),
			WantProcessed: 3,
			WantArgoCalls: 3,
			WantRowsAfter: 3,
		},
	}
	for _, tc := range tcs {
		t.Run(tc.Name, func(t *testing.T) {
			ctx := context.Background()
			dbHandler := setupDB(t)

			errW := dbHandler.WithTransaction(ctx, false, func(ctx context.Context, tx *sql.Tx) error {
				for i := 0; i < tc.SeedRows; i++ {
					name := "app" + string(rune('1'+i))
					if err := dbHandler.UpsertRolloutUndeployCascade(ctx, tx, name, "staging", false, 0); err != nil {
						return err
					}
				}
				return nil
			})
			if errW != nil {
				t.Fatalf("seed: %v", errW)
			}

			mock := &mockAppDeleter{deleteCalls: nil, deleteErr: tc.DeleteErr}
			processed, err := processOneBatch(ctx, dbHandler, mock, new(atomic.Int64))
			if err != nil {
				t.Fatalf("processOneBatch: %v", err)
			}
			if processed != tc.WantProcessed {
				t.Errorf("processed: want %d, got %d", tc.WantProcessed, processed)
			}
			if len(mock.deleteCalls) != tc.WantArgoCalls {
				t.Errorf("Argo Delete calls: want %d, got %d", tc.WantArgoCalls, len(mock.deleteCalls))
			}

			var rowsAfter []*db.RolloutShouldUndeployCascade
			errR := dbHandler.WithTransaction(ctx, true, func(ctx context.Context, tx *sql.Tx) error {
				got, err := dbHandler.DBReadRolloutUndeployCascadeBatch(ctx, tx, batchSize+10)
				if err != nil {
					return err
				}
				rowsAfter = got
				return nil
			})
			if errR != nil {
				t.Fatalf("read after: %v", errR)
			}
			if len(rowsAfter) != tc.WantRowsAfter {
				t.Errorf("rows after batch: want %d, got %d", tc.WantRowsAfter, len(rowsAfter))
			}
		})
	}
}

// TestProcessOneBatch_GatingEslId verifies that a cascade row with
// gating_transformer_esl_id > maxSeenTransformerEslId is NOT processed.
func TestProcessOneBatch_GatingEslId(t *testing.T) {
	tcs := []struct {
		Name                    string
		GatingTransformerEslId  db.TransformerID
		MaxSeenTransformerEslId int64
		WantArgoCalls           int
		WantRowGone             bool
	}{
		{
			Name:                    "gated: gating > max_seen, row must be skipped",
			GatingTransformerEslId:  100,
			MaxSeenTransformerEslId: 0,
			WantArgoCalls:           0,
			WantRowGone:             false,
		},
		{
			Name:                    "allowed: gating == max_seen, row must be processed",
			GatingTransformerEslId:  5,
			MaxSeenTransformerEslId: 5,
			WantArgoCalls:           1,
			WantRowGone:             true,
		},
		{
			Name:                    "allowed: gating < max_seen, row must be processed",
			GatingTransformerEslId:  3,
			MaxSeenTransformerEslId: 10,
			WantArgoCalls:           1,
			WantRowGone:             true,
		},
	}
	for _, tc := range tcs {
		t.Run(tc.Name, func(t *testing.T) {
			ctx := context.Background()
			dbHandler := setupDB(t)

			errW := dbHandler.WithTransaction(ctx, false, func(ctx context.Context, tx *sql.Tx) error {
				return dbHandler.UpsertRolloutUndeployCascade(ctx, tx, "my-app", "staging", false, tc.GatingTransformerEslId)
			})
			if errW != nil {
				t.Fatalf("seed: %v", errW)
			}

			mock := &mockAppDeleter{}
			maxProcessed := &atomic.Int64{}
			maxProcessed.Store(tc.MaxSeenTransformerEslId)
			processed, err := processOneBatch(ctx, dbHandler, mock, maxProcessed)
			if err != nil {
				t.Fatalf("processOneBatch: %v", err)
			}

			if len(mock.deleteCalls) != tc.WantArgoCalls {
				t.Errorf("Argo Delete calls: want %d, got %d (processed=%d)", tc.WantArgoCalls, len(mock.deleteCalls), processed)
			}

			var rowsAfter []*db.RolloutShouldUndeployCascade
			errR := dbHandler.WithTransaction(ctx, true, func(ctx context.Context, tx *sql.Tx) error {
				got, err := dbHandler.DBReadRolloutUndeployCascadeBatch(ctx, tx, 10)
				rowsAfter = got
				return err
			})
			if errR != nil {
				t.Fatalf("read after: %v", errR)
			}
			if tc.WantRowGone && len(rowsAfter) != 0 {
				t.Errorf("expected row removed, but %d rows remain", len(rowsAfter))
			}
			if !tc.WantRowGone && len(rowsAfter) != 1 {
				t.Errorf("expected row to remain, but got %d rows", len(rowsAfter))
			}
		})
	}
}

// TestProcessOneBatch_SkipsStaleRows verifies that processOneBatch does NOT issue a
// cascade Delete when the app/bracket has been repopulated since the cascade row was
// written (Option 1 safety check). This protects against the rollout-service being
// offline for a while, during which a bracket/app was undeployed AND redeployed, and
// then an old stale cascade row firing and destroying the live workload.
//
// For a plain-app row (is_bracket=false): if the app has an active deployment in that
// env, the row is stale → skip Delete, remove row.
// For a bracket row (is_bracket=true): if any member of the bracket has a deployment
// in that env, the row is stale → skip Delete, remove row.
func TestProcessOneBatch_SkipsStaleRows(t *testing.T) {
	tcs := []struct {
		Name      string
		IsBracket bool
	}{
		{
			Name:      "stale plain-app row: app has active deployment, cascade skipped",
			IsBracket: false,
		},
		{
			Name:      "stale bracket row: bracket member has active deployment, cascade skipped",
			IsBracket: true,
		},
	}
	for _, tc := range tcs {
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()
			ctx := context.Background()
			dbHandler := setupDB(t)
			const appName types.AppName = "my-app"
			const bracketName types.ArgoBracketName = "my-bracket"
			const envName types.EnvName = "staging"
			v := uint64(1)

			err := dbHandler.WithTransaction(ctx, false, func(ctx context.Context, tx *sql.Tx) error {
				// Seed one ESL event — brackets_history has a FK to event_sourcing_light.
				if err := dbHandler.DBWriteEslEventInternal(ctx, "empty", tx, interface{}(nil), db.ESLMetadata{}); err != nil {
					return err
				}
				// Seed a release so the deployment FK is satisfied.
				rel := db.DBReleaseWithMetaData{
					App:            appName,
					ReleaseNumbers: types.MakeReleaseNumbers(v, 0),
					Manifests:      db.DBReleaseManifests{Manifests: map[types.EnvName]string{}},
					Environments:   []types.EnvName{},
				} //exhaustruct:ignore
				if err := dbHandler.DBUpdateOrCreateRelease(ctx, tx, rel); err != nil {
					return err
				}
				// Seed an active deployment so the cascade row is "stale".
				dep := db.Deployment{
					App:            appName,
					Env:            envName,
					ReleaseNumbers: types.MakeReleaseNumbers(v, 0),
				} //exhaustruct:ignore
				if err := dbHandler.DBUpdateOrCreateDeployment(ctx, tx, dep); err != nil {
					return err
				}
				if tc.IsBracket {
					// Also seed bracket membership: appName is a member of bracketName.
					now := time.Now()
					if err := db.HandleBracketsHistoryUpdate(ctx, dbHandler, tx, appName, db.ResolveBracketName(appName, bracketName), now, 1); err != nil {
						return err
					}
					return dbHandler.UpsertRolloutUndeployCascade(ctx, tx, string(bracketName), envName, true, 0)
				}
				return dbHandler.UpsertRolloutUndeployCascade(ctx, tx, string(appName), envName, false, 0)
			})
			if err != nil {
				t.Fatalf("seed: %v", err)
			}

			mock := &mockAppDeleter{}
			maxProcessed := &atomic.Int64{}
			maxProcessed.Store(999) // ESL gate wide open
			_, batchErr := processOneBatch(ctx, dbHandler, mock, maxProcessed)
			if batchErr != nil {
				t.Fatalf("processOneBatch: %v", batchErr)
			}

			if len(mock.deleteCalls) != 0 {
				t.Errorf("expected 0 Argo Delete calls (row is stale), got %d", len(mock.deleteCalls))
			}

			// The stale row must be removed so it doesn't clog the queue.
			var rowsAfter []*db.RolloutShouldUndeployCascade
			if err := dbHandler.WithTransaction(ctx, true, func(ctx context.Context, tx *sql.Tx) error {
				rows, err := dbHandler.DBReadRolloutUndeployCascadeBatch(ctx, tx, 10)
				rowsAfter = rows
				return err
			}); err != nil {
				t.Fatalf("read after: %v", err)
			}
			if len(rowsAfter) != 0 {
				t.Errorf("expected stale row removed from queue, got %d rows remaining", len(rowsAfter))
			}
		})
	}
}

// setupDB mirrors the pattern in pkg/db/db_test.go (unexported there) and
// services/rollout-service/pkg/service/service_test.go (SetupDB). Kept
// undeploy-test-local so this package owns its own minimal setup.
func setupDB(t *testing.T) *db.DBHandler {
	ctx := context.Background()
	migrationsPath, err := db.CreateMigrationsPath(4)
	if err != nil {
		t.Fatalf("migrationspath: %v", err)
	}
	dbConfig, err := db.ConnectToPostgresContainer(ctx, t, migrationsPath, t.Name())
	if err != nil {
		t.Fatalf("ConnectToPostgresContainer: %v", err)
	}
	if err := db.RunDBMigrations(ctx, *dbConfig); err != nil {
		t.Fatalf("RunDBMigrations: %v", err)
	}
	dbHandler, err := db.Connect(ctx, *dbConfig)
	if err != nil {
		t.Fatalf("db.Connect: %v", err)
	}
	return dbHandler
}
