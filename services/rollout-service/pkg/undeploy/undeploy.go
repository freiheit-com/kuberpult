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

// Package undeploy drains the rollout_should_undeploy_cascade work queue: for
// every (argo_app, env) row written by the cd-service on a real undeploy, it
// issues a cascade=true delete on the corresponding Argo CD Application so that
// the workload k8s resources are cleaned up.
//
// The rollout-service's other delete paths (in argo.ProcessAppChange) use
// cascade=false; they never destroy workload resources. Cascade=true happens
// only here, with an explicit DB intent from the cd-service.
package undeploy

import (
	"context"
	"database/sql"
	"fmt"
	"sync/atomic"
	"time"

	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/freiheit-com/kuberpult/pkg/db"
	"github.com/freiheit-com/kuberpult/pkg/logger"
	"github.com/freiheit-com/kuberpult/pkg/setup"
	"github.com/freiheit-com/kuberpult/services/rollout-service/pkg/argo"
)

const (
	// batchSize caps each iteration's read so we never load an unbounded queue.
	batchSize = 50
	// pollInterval is the sleep between iterations when the queue is empty.
	// When a batch comes back full we loop again with no sleep so a backlog
	// drains as fast as Argo CD can accept the deletes.
	pollInterval = 10 * time.Second
	// maxAttempts caps how often we retry a single row. After this many failed
	// cascade-deletes, we drop the row and log loudly so a human can re-trigger
	// the undeploy. The intent itself is durably preserved in apps_history.
	maxAttempts = 5
)

// ConsumeUndeployCascade is the BackgroundFunc registered in cmd/server.go.
// It loops forever (until ctx is cancelled), polling the queue and processing
// rows in batches. Errors at the iteration level are reported through the
// HealthReporter so the setup framework can back off and eventually fail the
// process if the DB or Argo CD is persistently broken.
//
// appClient is narrowed to argo.ApplicationDeleter so the unit test can pass a
// minimal mock. The cascade-delete itself goes through argo.DeleteApplication,
// the single Delete-RPC caller in the rollout-service.
func ConsumeUndeployCascade(
	ctx context.Context,
	dbHandler *db.DBHandler,
	appClient argo.ApplicationDeleter,
	maxProcessedTransformerEslId *atomic.Int64,
	health *setup.HealthReporter,
) error {
	return health.Retry(ctx, func() error {
		health.ReportReady("polling")
		for {
			select {
			case <-ctx.Done():
				return setup.Permanent(nil)
			default:
			}
			processed, err := processOneBatch(ctx, dbHandler, appClient, maxProcessedTransformerEslId)
			if err != nil {
				return err
			}
			if processed < batchSize {
				// queue drained — wait before polling again
				select {
				case <-ctx.Done():
					return setup.Permanent(nil)
				case <-time.After(pollInterval):
				}
			}
			// queue still full — loop immediately
		}
	})
}

// processOneBatch reads up to batchSize rows in one transaction, releases the
// transaction, then processes each eligible row in its own transaction. Returns
// the number of rows read (including gated ones) so the caller can decide
// whether to loop or sleep.
func processOneBatch(ctx context.Context, dbHandler *db.DBHandler, appClient argo.ApplicationDeleter, maxProcessedTransformerEslId *atomic.Int64) (int, error) {
	var batch []*db.RolloutShouldUndeployCascade
	err := dbHandler.WithTransaction(ctx, true, func(ctx context.Context, tx *sql.Tx) error {
		rows, err := dbHandler.DBReadRolloutUndeployCascadeBatch(ctx, tx, batchSize)
		if err != nil {
			return err
		}
		batch = rows
		return nil
	})
	if err != nil {
		return 0, fmt.Errorf("undeploy: read batch: %w", err)
	}
	maxProcessed := maxProcessedTransformerEslId.Load()
	for _, row := range batch {
		if int64(row.NotBeforeTransformerEslId) > maxProcessed {
			// The rollout-service has not yet fully processed the gRPC event
			// that corresponds to this cascade row. Skip for now; the next
			// poll (after Consume advances maxProcessedTransformerEslId) will
			// pick it up.
			continue
		}
		processRow(ctx, dbHandler, appClient, row)
	}
	return len(batch), nil
}

// processRow does the Argo CD Delete for a single row and updates the table
// accordingly. Errors are logged but not returned: one bad row must not stall
// the rest of the batch.
func processRow(ctx context.Context, dbHandler *db.DBHandler, appClient argo.ApplicationDeleter, row *db.RolloutShouldUndeployCascade) {
	argoAppName := string(row.Env) + "-" + row.ArgoApp
	l := logger.FromContext(ctx).With(
		zap.String("argo.app", argoAppName),
		zap.String("env", string(row.Env)),
		zap.Int("attempts", row.Attempts),
	)
	err := argo.DeleteApplication(ctx, appClient, argoAppName, true)
	switch {
	case err == nil:
		l.Info("rollout.undeploy.cascade.deleted")
		if dbErr := deleteRow(ctx, dbHandler, row); dbErr != nil {
			l.Error("rollout.undeploy.cascade.row-delete-failed", zap.Error(dbErr))
		}
	case status.Code(err) == codes.NotFound:
		// Argo Application is already gone — idempotent success.
		l.Info("rollout.undeploy.cascade.already-gone")
		if dbErr := deleteRow(ctx, dbHandler, row); dbErr != nil {
			l.Error("rollout.undeploy.cascade.row-delete-failed", zap.Error(dbErr))
		}
	default:
		// Transient. Bump attempts; drop the row once the budget is exhausted.
		nextAttempt := row.Attempts + 1
		if nextAttempt >= maxAttempts {
			l.Error("rollout.undeploy.cascade.attempts-exhausted",
				zap.Int("max-attempts", maxAttempts),
				zap.Error(err))
			if dbErr := deleteRow(ctx, dbHandler, row); dbErr != nil {
				l.Error("rollout.undeploy.cascade.row-delete-failed", zap.Error(dbErr))
			}
			return
		}
		l.Warn("rollout.undeploy.cascade.retry", zap.Error(err), zap.Int("nextAttempt", nextAttempt))
		if dbErr := incrementAttempts(ctx, dbHandler, row); dbErr != nil {
			l.Error("rollout.undeploy.cascade.attempts-increment-failed", zap.Error(dbErr))
		}
	}
}

func deleteRow(ctx context.Context, dbHandler *db.DBHandler, row *db.RolloutShouldUndeployCascade) error {
	return dbHandler.WithTransaction(ctx, false, func(ctx context.Context, tx *sql.Tx) error {
		return dbHandler.DBDeleteRolloutUndeployCascade(ctx, tx, row.ArgoApp, row.Env)
	})
}

func incrementAttempts(ctx context.Context, dbHandler *db.DBHandler, row *db.RolloutShouldUndeployCascade) error {
	return dbHandler.WithTransaction(ctx, false, func(ctx context.Context, tx *sql.Tx) error {
		return dbHandler.DBIncrementRolloutUndeployCascadeAttempts(ctx, tx, row.ArgoApp, row.Env)
	})
}
