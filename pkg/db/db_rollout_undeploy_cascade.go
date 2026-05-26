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
	"time"

	"gopkg.in/DataDog/dd-trace-go.v1/ddtrace/tracer"

	"github.com/freiheit-com/kuberpult/pkg/types"
)

/*
rollout_should_undeploy_cascade is a work-queue between the cd-service and the
rollout-service. Each row signals: "the Argo Application named <env>-<argo_app>
in env <env> represents an undeployed kuberpult entity and should be deleted
from Argo CD with cascade=true so its k8s resources are cleaned up."

The cd-service writes rows when the UndeployApplication / DeleteEnvFromApp
transformers run. The rollout-service polls the table in a background task,
issues the cascade-delete, and removes the row. If the delete keeps failing,
the attempts counter is bumped; once it crosses the budget, the row is removed
and the failure is logged at error level for human follow-up.

argo_app is the value of the com.freiheit.kuberpult/application annotation on
the Argo Application — i.e. the kuberpult app name for an individual app, or
the bracket name for a bracket app. The Argo CD Application name is the
concatenation <env>-<argo_app>.
*/
const rolloutShouldUndeployCascadeTable = "rollout_should_undeploy_cascade"

type RolloutShouldUndeployCascade struct {
	ArgoApp  string
	Env      types.EnvName
	Created  time.Time
	Attempts int
}

// UpsertRolloutUndeployCascade inserts a pending cascade-delete row.
// Used by the cd-service in UndeployApplication / DeleteEnvFromApp transformers.
// ON CONFLICT DO NOTHING — re-triggering an undeploy must not reset the attempt
// counter, which would let a permanently-failing row loop forever.
func (h *DBHandler) UpsertRolloutUndeployCascade(ctx context.Context, tx *sql.Tx, argoApp string, env types.EnvName) (err error) {
	span, ctx := tracer.StartSpanFromContext(ctx, "UpsertRolloutUndeployCascade")
	defer func() {
		span.Finish(tracer.WithError(err))
	}()
	upsertQuery := h.AdaptQuery(`
		INSERT INTO ` + rolloutShouldUndeployCascadeTable + ` (argo_app, env)
		VALUES (?, ?)
		ON CONFLICT(argo_app, env) DO NOTHING;
	`)
	span.SetTag("query", upsertQuery)
	_, err = tx.ExecContext(ctx, upsertQuery, argoApp, env)
	if err != nil {
		return fmt.Errorf("could not insert rollout undeploy cascade row for argo_app '%s' env '%s': %w", argoApp, env, err)
	}
	return nil
}

// DBReadRolloutUndeployCascadeBatch returns up to `limit` pending rows
// ordered by created ASC (oldest first). Used by the rollout-service consumer.
func (h *DBHandler) DBReadRolloutUndeployCascadeBatch(ctx context.Context, tx *sql.Tx, limit int) (_ []*RolloutShouldUndeployCascade, err error) {
	span, ctx := tracer.StartSpanFromContext(ctx, "DBReadRolloutUndeployCascadeBatch")
	defer func() {
		span.Finish(tracer.WithError(err))
	}()
	selectQuery := h.AdaptQuery(`
		SELECT created, argo_app, env, attempts
		FROM ` + rolloutShouldUndeployCascadeTable + `
		ORDER BY created ASC, argo_app ASC, env ASC
		LIMIT ?;
	`)
	span.SetTag("query", selectQuery)
	rows, err := tx.QueryContext(ctx, selectQuery, limit)
	if err != nil {
		return nil, fmt.Errorf("could not read rollout_should_undeploy_cascade batch: %w", err)
	}
	defer closeRowsAndLog(rows, ctx, "DBReadRolloutUndeployCascadeBatch")

	result := make([]*RolloutShouldUndeployCascade, 0)
	for rows.Next() {
		row := RolloutShouldUndeployCascade{
			ArgoApp:  "",
			Env:      "",
			Created:  time.Time{},
			Attempts: 0,
		}
		if err := rows.Scan(&row.Created, &row.ArgoApp, &row.Env, &row.Attempts); err != nil {
			return nil, fmt.Errorf("could not scan rollout_should_undeploy_cascade row: %w", err)
		}
		result = append(result, &row)
	}
	return result, nil
}

// DBDeleteRolloutUndeployCascade removes a row from the work queue.
// Used by the rollout-service after a successful cascade-delete, after a
// NotFound (already gone — idempotent), or after the attempts budget is
// exhausted.
func (h *DBHandler) DBDeleteRolloutUndeployCascade(ctx context.Context, tx *sql.Tx, argoApp string, env types.EnvName) (err error) {
	span, ctx := tracer.StartSpanFromContext(ctx, "DBDeleteRolloutUndeployCascade")
	defer func() {
		span.Finish(tracer.WithError(err))
	}()
	deleteQuery := h.AdaptQuery(`
		DELETE FROM ` + rolloutShouldUndeployCascadeTable + `
		WHERE argo_app = ? AND env = ?;
	`)
	span.SetTag("query", deleteQuery)
	_, err = tx.ExecContext(ctx, deleteQuery, argoApp, env)
	if err != nil {
		return fmt.Errorf("could not delete rollout undeploy cascade row for argo_app '%s' env '%s': %w", argoApp, env, err)
	}
	return nil
}

// DBIncrementRolloutUndeployCascadeAttempts bumps the attempt counter on a
// transient Argo CD error. The rollout-service compares attempts against a
// budget after this call; once the budget is exhausted, the row is removed
// via DBDeleteRolloutUndeployCascade and the failure is logged.
func (h *DBHandler) DBIncrementRolloutUndeployCascadeAttempts(ctx context.Context, tx *sql.Tx, argoApp string, env types.EnvName) (err error) {
	span, ctx := tracer.StartSpanFromContext(ctx, "DBIncrementRolloutUndeployCascadeAttempts")
	defer func() {
		span.Finish(tracer.WithError(err))
	}()
	updateQuery := h.AdaptQuery(`
		UPDATE ` + rolloutShouldUndeployCascadeTable + `
		SET attempts = attempts + 1
		WHERE argo_app = ? AND env = ?;
	`)
	span.SetTag("query", updateQuery)
	_, err = tx.ExecContext(ctx, updateQuery, argoApp, env)
	if err != nil {
		return fmt.Errorf("could not increment attempts for rollout undeploy cascade row argo_app '%s' env '%s': %w", argoApp, env, err)
	}
	return nil
}
