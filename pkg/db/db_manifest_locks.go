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
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"gopkg.in/DataDog/dd-trace-go.v1/ddtrace/tracer"

	"github.com/freiheit-com/kuberpult/pkg/types"
)

type ManifestLockEventType string

const (
	ManifestLockEventTypeCreated ManifestLockEventType = "created"
	ManifestLockEventTypeDeleted ManifestLockEventType = "deleted"
)

type ManifestLock struct {
	LockID     types.ManifestLockID
	RecordedAt time.Time
	App        types.AppName
	Env        types.EnvName
	Metadata   LockMetadata
	Active     bool
	EventType  ManifestLockEventType
}

// SELECTS

func (h *DBHandler) DBHasActiveManifestLock(ctx context.Context, tx *sql.Tx, app types.AppName, env types.EnvName) (_ bool, err error) {
	span, ctx := tracer.StartSpanFromContext(ctx, "DBHasActiveManifestLock")
	defer func() {
		span.Finish(tracer.WithError(err))
	}()

	selectQuery := h.AdaptQuery(`
		SELECT EXISTS(
			SELECT 1 FROM manifest_locks_history
			WHERE app = ? AND env = ? AND active = true
		);`)

	rows, err := tx.QueryContext(ctx, selectQuery, app, env)
	if err != nil {
		return false, fmt.Errorf("could not query manifest_locks_history: %w", err)
	}
	defer closeRowsAndLog(rows, ctx, "DBHasActiveManifestLock")
	if !rows.Next() {
		return false, nil
	}
	var exists bool
	if err := rows.Scan(&exists); err != nil {
		return false, fmt.Errorf("error scanning DBHasActiveManifestLock result: %w", err)
	}
	return exists, nil
}

func (h *DBHandler) DBSelectAllActiveManifestLocks(ctx context.Context, tx *sql.Tx) (_ []ManifestLock, err error) {
	span, ctx := tracer.StartSpanFromContext(ctx, "DBSelectAllActiveManifestLocks")
	defer func() {
		span.Finish(tracer.WithError(err))
	}()

	selectQuery := h.AdaptQuery(`
		SELECT lockid, recorded_at, app, env, metadata, active, event_type
		FROM manifest_locks_history
		WHERE active = true
		ORDER BY env, app;`)

	rows, err := tx.QueryContext(ctx, selectQuery)
	if err != nil {
		return nil, fmt.Errorf("could not query manifest_locks_history table from DB. Error: %w", err)
	}
	defer closeRowsAndLog(rows, ctx, "DBSelectAllActiveManifestLocks")
	return h.processManifestLockRows(ctx, rows)
}

func (h *DBHandler) DBSelectAllActiveManifestLocksForApp(ctx context.Context, tx *sql.Tx, appName types.AppName) (_ []ManifestLock, err error) {
	span, ctx := tracer.StartSpanFromContext(ctx, "DBSelectAllActiveManifestLocksForApp")
	defer func() {
		span.Finish(tracer.WithError(err))
	}()

	selectQuery := h.AdaptQuery(`
		SELECT lockid, recorded_at, app, env, metadata, active, event_type
		FROM manifest_locks_history
		WHERE active = true AND app = ?
		ORDER BY env;`)

	rows, err := tx.QueryContext(ctx, selectQuery, appName)
	if err != nil {
		return nil, fmt.Errorf("could not query manifest_locks_history table from DB. Error: %w", err)
	}
	defer closeRowsAndLog(rows, ctx, "DBSelectAllActiveManifestLocksForApp")
	return h.processManifestLockRows(ctx, rows)
}

// INSERT, UPDATE

func (h *DBHandler) DBWriteManifestLock(ctx context.Context, tx *sql.Tx, app types.AppName, env types.EnvName, metadata LockMetadata) (err error) {
	span, ctx := tracer.StartSpanFromContext(ctx, "DBWriteManifestLock")
	defer func() {
		span.Finish(tracer.WithError(err))
	}()

	now, err := h.DBReadTransactionTimestamp(ctx, tx)
	if err != nil {
		return fmt.Errorf("DBWriteManifestLock unable to get transaction timestamp: %w", err)
	}

	metadataJSON, err := json.Marshal(metadata)
	if err != nil {
		return fmt.Errorf("DBWriteManifestLock could not marshal metadata: %w", err)
	}

	insertQuery := h.AdaptQuery(`
		INSERT INTO manifest_locks_history (recorded_at, app, env, metadata, active, event_type)
		VALUES (?, ?, ?, ?, true, ?);`)

	_, err = tx.ExecContext(ctx, insertQuery, *now, app, env, metadataJSON, ManifestLockEventTypeCreated)
	if err != nil {
		return fmt.Errorf("could not insert manifest lock for app '%s' env '%s' into DB. Error: %w", app, env, err)
	}
	return nil
}

func (h *DBHandler) DBDeleteManifestLock(ctx context.Context, tx *sql.Tx, app types.AppName, env types.EnvName) (err error) {
	span, ctx := tracer.StartSpanFromContext(ctx, "DBDeleteManifestLock")
	defer func() {
		span.Finish(tracer.WithError(err))
	}()

	updateQuery := h.AdaptQuery(`
		UPDATE manifest_locks_history
		SET active = false, event_type = ?
		WHERE app = ? AND env = ? AND active = true;`)

	_, err = tx.ExecContext(ctx, updateQuery, ManifestLockEventTypeDeleted, app, env)
	if err != nil {
		return fmt.Errorf("could not delete manifest lock for app '%s' env '%s' from DB. Error: %w", app, env, err)
	}
	return nil
}

// process rows

func (h *DBHandler) processManifestLockRows(ctx context.Context, rows *sql.Rows) ([]ManifestLock, error) {
	var locks []ManifestLock
	for rows.Next() {
		var row ManifestLock
		var metadataJSON string
		var eventType string
		err := rows.Scan(&row.LockID, &row.RecordedAt, &row.App, &row.Env, &metadataJSON, &row.Active, &eventType)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return nil, nil
			}
			return nil, fmt.Errorf("error scanning manifest_locks_history row from DB. Error: %w", err)
		}
		row.EventType = ManifestLockEventType(eventType)

		//exhaustruct:ignore
		err = json.Unmarshal([]byte(metadataJSON), &row.Metadata)
		if err != nil {
			return nil, fmt.Errorf("error during json unmarshal of manifest lock metadata. Error: %w", err)
		}
		locks = append(locks, row)
	}
	err := closeRows(rows)
	if err != nil {
		return nil, err
	}
	return locks, nil
}
