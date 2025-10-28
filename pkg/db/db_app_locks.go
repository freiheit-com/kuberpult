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
	"strings"
	"time"

	"github.com/freiheit-com/kuberpult/pkg/logger"
	"github.com/freiheit-com/kuberpult/pkg/types"
	"gopkg.in/DataDog/dd-trace-go.v1/ddtrace/tracer"
)

type ApplicationLock struct {
	Created  time.Time
	LockID   string
	Env      types.EnvName
	App      string
	Metadata LockMetadata
}

type ApplicationLockHistory struct {
	Created          time.Time
	LockID           string
	Env              types.EnvName
	App              string
	Metadata         LockMetadata
	Deleted          bool
	DeletionMetadata LockDeletionMetadata
}

// SELECTS

func (h *DBHandler) DBSelectAppLock(ctx context.Context, tx *sql.Tx, environment types.EnvName, appName string, lockID string) (*ApplicationLockHistory, error) {
	selectQuery := h.AdaptQuery(`
		SELECT created, lockID, envName, appName, metadata, deleted
		FROM app_locks_history
		WHERE envName=? AND appName=? AND lockID=?
		ORDER BY version DESC
		LIMIT 1;`)

	rows, err := tx.QueryContext(
		ctx,
		selectQuery,
		environment,
		appName,
		lockID,
	)
	if err != nil {
		return nil, fmt.Errorf("could not query application locks table from DB. Error: %w", err)
	}
	defer closeRowsAndLog(rows, ctx, "DBSelectAppLock")

	if rows.Next() {
		var row = ApplicationLockHistory{
			Created: time.Time{},
			LockID:  "",
			Env:     "",
			App:     "",
			Deleted: true,
			Metadata: LockMetadata{
				CreatedByName:     "",
				CreatedByEmail:    "",
				Message:           "",
				CiLink:            "",
				CreatedAt:         time.Time{},
				SuggestedLifeTime: "",
			},
		}
		var metaData string

		err := rows.Scan(&row.Created, &row.LockID, &row.Env, &row.App, &metaData, &row.Deleted)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return nil, nil
			}
			return nil, fmt.Errorf("error scanning application locks row from DB. Error: %w", err)
		}

		//exhaustruct:ignore
		err = json.Unmarshal(([]byte)(metaData), &row.Metadata)
		if err != nil {
			return nil, fmt.Errorf("error during json unmarshal. Error: %w. Data: %s", err, row.Metadata)
		}
		err = closeRows(rows)
		if err != nil {
			return nil, err
		}
		return &row, nil
	}

	return nil, nil // no rows, but also no error
}

func (h *DBHandler) DBSelectAllActiveAppLocksForApp(ctx context.Context, tx *sql.Tx, appName string) (_ []ApplicationLock, err error) {
	span, ctx := tracer.StartSpanFromContext(ctx, "DBSelectAllActiveAppLocksForApp")
	defer span.Finish(tracer.WithError(err))

	if h == nil {
		return nil, nil
	}
	if tx == nil {
		return nil, fmt.Errorf("DBSelectAllActiveAppLocksForApp: no transaction provided")
	}
	selectQuery := h.AdaptQuery(`
		SELECT created, lockid, envname, appname, metadata
		FROM app_locks
		WHERE appName = (?)
		ORDER BY lockId;`)

	rows, err := tx.QueryContext(ctx, selectQuery, appName)
	defer closeRowsAndLog(rows, ctx, "DBSelectAllActiveAppLocksForApp")
	return h.processAppLockRows(ctx, err, rows)
}

func (h *DBHandler) DBSelectAllActiveAppLocksForSliceApps(ctx context.Context, tx *sql.Tx, appNames []string) (_ []ApplicationLock, err error) {
	span, ctx := tracer.StartSpanFromContext(ctx, "DBSelectAllActiveAppLocksForSliceApps")
	defer span.Finish(tracer.WithError(err))

	if h == nil {
		return nil, nil
	}
	if tx == nil {
		return nil, fmt.Errorf("DBSelectAllActiveAppLocksForSliceApps: no transaction provided")
	}
	if len(appNames) == 0 {
		return []ApplicationLock{}, nil
	}
	selectQuery := h.AdaptQuery(`
		SELECT created, lockid, envname, appname, metadata
		FROM app_locks
		WHERE app_locks.appName IN (?` + strings.Repeat(",?", len(appNames)-1) + `)
		ORDER BY lockId;`)

	//Get the latest change to each lock
	args := []any{}
	for _, app := range appNames {
		args = append(args, app)
	}
	rows, err := tx.QueryContext(
		ctx,
		selectQuery,
		args...,
	)
	if err != nil {
		return nil, err
	}
	defer closeRowsAndLog(rows, ctx, "DBSelectAllActiveAppLocksForSliceApps")
	return h.processAppLockRows(ctx, err, rows)
}

func (h *DBHandler) DBSelectAppLockSet(ctx context.Context, tx *sql.Tx, environment types.EnvName, appName string, lockIDs []string) (_ []ApplicationLock, err error) {
	span, ctx := tracer.StartSpanFromContext(ctx, "DBSelectAppLockSet")
	defer span.Finish(tracer.WithError(err))

	if len(lockIDs) == 0 {
		return nil, nil
	}
	if h == nil {
		return nil, nil
	}
	if tx == nil {
		return nil, fmt.Errorf("DBSelectAppLockSet: no transaction provided")
	}

	var appLocks []ApplicationLock
	var rows *sql.Rows
	defer func(rows *sql.Rows) {
		if rows == nil {
			return
		}
		err := rows.Close()
		if err != nil {
			logger.FromContext(ctx).Sugar().Warnf("row closing error: %v", err)
		}
	}(rows)
	//Get the latest change to each lock
	for _, id := range lockIDs {
		var err error
		selectQuery := h.AdaptQuery(`
			SELECT created, lockID, envName, appName, metadata
			FROM app_locks
			WHERE envName=? AND lockID=? AND appName=?
			LIMIT 1;`)
		rows, err = tx.QueryContext(ctx, selectQuery, environment, id, appName)
		if err != nil {
			return nil, fmt.Errorf("could not query application locks table from DB. Error: %w", err)
		}

		var row = ApplicationLock{
			Created: time.Time{},
			LockID:  "",
			Env:     "",
			App:     "",
			Metadata: LockMetadata{
				CreatedByName:     "",
				CreatedByEmail:    "",
				Message:           "",
				CiLink:            "",
				CreatedAt:         time.Time{},
				SuggestedLifeTime: "",
			},
		}
		var metaData string
		if rows.Next() {
			err = rows.Scan(&row.Created, &row.LockID, &row.Env, &row.App, &metaData)
			if err != nil {
				if errors.Is(err, sql.ErrNoRows) {
					return nil, nil
				}
				return nil, fmt.Errorf("error scanning application locks row from DB. Error: %w", err)
			}

			//exhaustruct:ignore
			var resultJson = LockMetadata{}
			err = json.Unmarshal(([]byte)(metaData), &resultJson)
			if err != nil {
				return nil, fmt.Errorf("error during json unmarshal. Error: %w. Data: %s", err, row.Metadata)
			}
			appLocks = append(appLocks, ApplicationLock{
				Created:  row.Created,
				LockID:   row.LockID,
				Env:      row.Env,
				App:      row.App,
				Metadata: resultJson,
			})
		}
		err = closeRows(rows)
		if err != nil {
			return nil, err
		}
	}
	err = closeRows(rows)
	if err != nil {
		return nil, err
	}
	return appLocks, nil
}

func (h *DBHandler) DBSelectAllAppLocks(ctx context.Context, tx *sql.Tx, environment types.EnvName, appName string) (_ []string, err error) {
	span, ctx := tracer.StartSpanFromContext(ctx, "DBSelectAllAppLocks")
	defer span.Finish(tracer.WithError(err))
	if h == nil {
		return nil, nil
	}
	if tx == nil {
		return nil, fmt.Errorf("DBSelectAllAppLocks: no transaction provided")
	}
	selectQuery := h.AdaptQuery(`
		SELECT lockId FROM app_locks 
		WHERE envname = ? AND appName = ?
		ORDER BY lockId;`)
	span.SetTag("query", selectQuery)

	rows, err := tx.QueryContext(ctx, selectQuery, environment, appName)
	if err != nil {
		return nil, fmt.Errorf("could not query all app locks table from DB. Error: %w", err)
	}
	defer closeRowsAndLog(rows, ctx, "DBSelectAllAppLocks")
	var results []string
	for rows.Next() {
		var lockId string
		err := rows.Scan(&lockId)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return nil, nil
			}
			return nil, fmt.Errorf("error scanning application locks row from DB. Error: %w", err)
		}
		if results == nil {
			results = make([]string, 0)
		}
		results = append(results, lockId)
		err = closeRows(rows)
		if err != nil {
			return nil, err
		}
	}
	return results, nil
}

func (h *DBHandler) DBHasAnyActiveAppLock(ctx context.Context, tx *sql.Tx) (bool, error) {
	selectQuery := h.AdaptQuery(`
		SELECT created, lockId, envName, appName, metadata
		FROM app_locks
		LIMIT 1
		ORDER BY lockId;
	`)

	rows, err := tx.QueryContext(
		ctx,
		selectQuery,
	)
	if err != nil {
		return false, fmt.Errorf("error querying for any app lock")
	}
	defer closeRowsAndLog(rows, ctx, "DBHasAnyActiceAppLock")
	return rows.Next(), nil
}

func (h *DBHandler) DBSelectAllAppLocksForEnv(ctx context.Context, tx *sql.Tx, environment types.EnvName) (_ []ApplicationLock, err error) {
	span, ctx := tracer.StartSpanFromContext(ctx, "DBSelectAllAppLocksForEnv")
	defer span.Finish(tracer.WithError(err))

	selectQuery := h.AdaptQuery(`
		SELECT created, lockId, envName, appName, metadata 
		FROM app_locks
		WHERE envname = (?)
		ORDER BY appName, lockId;`)
	span.SetTag("query", selectQuery)
	rows, err := tx.QueryContext(
		ctx,
		selectQuery,
		environment,
	)
	return h.processAppLockRows(ctx, err, rows)
}

// DBSelectAppLockHistory returns the last N events associated with some lock on some environment for some app. Currently only used in testing.
func (h *DBHandler) DBSelectAppLockHistory(ctx context.Context, tx *sql.Tx, environmentName types.EnvName, appName string, lockID string, limit int) (_ []ApplicationLockHistory, err error) {
	span, ctx := tracer.StartSpanFromContext(ctx, "DBSelectAppLockHistory")
	defer span.Finish(tracer.WithError(err))

	if h == nil {
		return nil, nil
	}
	if tx == nil {
		return nil, fmt.Errorf("DBSelectAppLockHistory: no transaction provided")
	}

	selectQuery := h.AdaptQuery(`
		SELECT created, lockID, envName, appName, metadata, deleted, deletionMetadata
		FROM app_locks_history
		WHERE envName=? AND lockID=? AND appName=?
		ORDER BY version DESC
		LIMIT ?;
	`)

	span.SetTag("query", selectQuery)
	rows, err := tx.QueryContext(
		ctx,
		selectQuery,
		environmentName,
		lockID,
		appName,
		limit,
	)
	if err != nil {
		return nil, fmt.Errorf("could not read application lock from DB. Error: %w", err)
	}
	defer closeRowsAndLog(rows, ctx, "DBSelectAppLockHistory")
	appLocks := make([]ApplicationLockHistory, 0)
	for rows.Next() {
		var row = ApplicationLockHistory{
			Created: time.Time{},
			LockID:  "",
			App:     "",
			Env:     "",
			Deleted: true,
			Metadata: LockMetadata{
				CreatedByName:     "",
				CreatedByEmail:    "",
				Message:           "",
				CiLink:            "",
				CreatedAt:         time.Time{},
				SuggestedLifeTime: "",
			},
			DeletionMetadata: LockDeletionMetadata{
				DeletedByUser:  "",
				DeletedByEmail: "",
			},
		}
		var metaData string
		var deletionMetadata string
		err := rows.Scan(&row.Created, &row.LockID, &row.Env, &row.App, &metaData, &row.Deleted, &deletionMetadata)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return nil, nil
			}
			return nil, fmt.Errorf("error scanning application locks row from DB. Error: %w", err)
		}

		//exhaustruct:ignore
		var resultJson = LockMetadata{}
		err = json.Unmarshal(([]byte)(metaData), &resultJson)
		if err != nil {
			return nil, fmt.Errorf("error during json unmarshal. Error: %w. Data: %s", err, row.Metadata)
		}
		row.Metadata = resultJson

		//exhaustruct:ignore
		var deletionMetadataResultJson = LockDeletionMetadata{}
		err = json.Unmarshal(([]byte)(deletionMetadata), &deletionMetadataResultJson)
		if err != nil {
			return nil, fmt.Errorf("error during json deletion metadata unmarshal. Error: %w. Data: %s", err, row.DeletionMetadata)
		}

		row.DeletionMetadata = deletionMetadataResultJson
		appLocks = append(appLocks, row)
	}
	return appLocks, nil
}

// INSERT, UPDATE, DELETES
func (h *DBHandler) DBWriteApplicationLock(ctx context.Context, tx *sql.Tx, lockID string, environment types.EnvName, appName string, metadata LockMetadata) (err error) {
	span, ctx := tracer.StartSpanFromContext(ctx, "DBWriteApplicationLock")
	defer span.Finish(tracer.WithError(err))

	if h == nil {
		return nil
	}
	if tx == nil {
		return fmt.Errorf("DBWriteApplicationLock: no transaction provided")
	}
	err = h.upsertAppLockRow(ctx, tx, lockID, environment, appName, metadata)
	if err != nil {
		return err
	}
	err = h.insertAppLockHistoryRow(ctx, tx, lockID, environment, appName, metadata, false, LockDeletionMetadata{DeletedByUser: "", DeletedByEmail: ""}) //Empty deletion metadata on insertion
	if err != nil {
		return err
	}
	return nil
}

func (h *DBHandler) DBDeleteApplicationLock(ctx context.Context, tx *sql.Tx, environment types.EnvName, appName, lockID string, deletionMetadata LockDeletionMetadata) (err error) {
	span, ctx := tracer.StartSpanFromContext(ctx, "DBDeleteApplicationLock")
	defer span.Finish(tracer.WithError(err))

	if h == nil {
		return nil
	}
	if tx == nil {
		return fmt.Errorf("DBDeleteApplicationLock: no transaction provided")
	}
	targetLock, err := h.DBSelectAppLock(ctx, tx, environment, appName, lockID)
	if err != nil {
		return err
	}
	if targetLock == nil {
		return nil
	}
	err = h.deleteAppLockRow(ctx, tx, lockID, environment, appName)
	if err != nil {
		return err
	}
	err = h.insertAppLockHistoryRow(ctx, tx, lockID, environment, appName, targetLock.Metadata, true, deletionMetadata)
	if err != nil {
		return err
	}
	return nil
}

// actual changes in tables

func (h *DBHandler) upsertAppLockRow(ctx context.Context, transaction *sql.Tx, lockID string, environment types.EnvName, appName string, metadata LockMetadata) (err error) {
	span, ctx := tracer.StartSpanFromContext(ctx, "upsertAppLockRow")
	defer span.Finish(tracer.WithError(err))
	upsertQuery := h.AdaptQuery(`
		INSERT INTO app_locks (created, lockId, envname, appName, metadata)
		VALUES (?, ?, ?, ?, ?)
		ON CONFLICT(appname, envname, lockid)
		DO UPDATE SET created = excluded.created, lockid = excluded.lockid, metadata = excluded.metadata, envname = excluded.envname, appname = excluded.appname;
	`)
	span.SetTag("query", upsertQuery)
	jsonToInsert, err := json.Marshal(metadata)
	if err != nil {
		return fmt.Errorf("could not marshal json data: %w", err)
	}

	now, err := h.DBReadTransactionTimestamp(ctx, transaction)
	if err != nil {
		return fmt.Errorf("upsertAppLockRow unable to get transaction timestamp: %w", err)
	}
	_, err = transaction.Exec(
		upsertQuery,
		*now,
		lockID,
		environment,
		appName,
		jsonToInsert,
	)
	if err != nil {
		return fmt.Errorf(
			"could not insert app lock for app '%s' and environment '%s' and lockid '%s' into DB. Error: %w",
			appName,
			environment,
			lockID,
			err)
	}
	return nil
}

func (h *DBHandler) deleteAppLockRow(ctx context.Context, transaction *sql.Tx, lockId string, environment types.EnvName, appname string) (err error) {
	span, _ := tracer.StartSpanFromContext(ctx, "deleteAppLockRow")
	defer span.Finish(tracer.WithError(err))
	deleteQuery := h.AdaptQuery(`
		DELETE FROM app_locks WHERE appname=? AND lockId=? AND envname=?
	`)
	span.SetTag("query", deleteQuery)
	_, err = transaction.Exec(
		deleteQuery,
		appname,
		lockId,
		environment,
	)
	if err != nil {
		return fmt.Errorf(
			"could not delete app_lock for app '%s' and environment '%s' and lockId '%s' from DB. Error: %w",
			appname,
			environment,
			lockId,
			err)
	}
	return nil
}

func (h *DBHandler) insertAppLockHistoryRow(ctx context.Context, transaction *sql.Tx, lockID string, environment types.EnvName, appName string, metadata LockMetadata, deleted bool, deletionMetadata LockDeletionMetadata) (err error) {
	span, ctx := tracer.StartSpanFromContext(ctx, "insertAppLockHistoryRow")
	defer span.Finish(tracer.WithError(err))
	upsertQuery := h.AdaptQuery(`
		INSERT INTO app_locks_history (created, lockId, envname, appName, metadata, deleted, deletionMetadata)
		VALUES (?, ?, ?, ?, ?, ?, ?);
	`)
	span.SetTag("query", upsertQuery)
	jsonToInsert, err := json.Marshal(metadata)
	if err != nil {
		return fmt.Errorf("could not marshal json data: %w", err)
	}

	deletionMetadataJsonToInsert, err := json.Marshal(deletionMetadata)
	if err != nil {
		return fmt.Errorf("could not marshal json deletion metadata: %w", err)
	}

	now, err := h.DBReadTransactionTimestamp(ctx, transaction)
	if err != nil {
		return fmt.Errorf("upsertAppLockRow unable to get transaction timestamp: %w", err)
	}
	_, err = transaction.Exec(
		upsertQuery,
		*now,
		lockID,
		environment,
		appName,
		jsonToInsert,
		deleted,
		deletionMetadataJsonToInsert,
	)
	if err != nil {
		return fmt.Errorf(
			"could not insert app lock history for app '%s' and environment '%s' and lockid '%s' into DB. Error: %w",
			appName,
			environment,
			lockID,
			err)
	}
	return nil
}

// process rows functions
func (h *DBHandler) processAppLockRows(ctx context.Context, err error, rows *sql.Rows) ([]ApplicationLock, error) {
	var appLocks []ApplicationLock
	if err != nil {
		return nil, fmt.Errorf("could not query application locks table from DB. Error: %w", err)
	}
	defer closeRowsAndLog(rows, ctx, "app locks")
	for rows.Next() {
		var row = ApplicationLock{
			Created: time.Time{},
			LockID:  "",
			Env:     "",
			App:     "",
			Metadata: LockMetadata{
				CreatedAt:         time.Time{},
				CreatedByEmail:    "",
				CreatedByName:     "",
				Message:           "",
				CiLink:            "",
				SuggestedLifeTime: "",
			},
		}
		var metadataJson string
		err := rows.Scan(&row.Created, &row.LockID, &row.Env, &row.App, &metadataJson)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return nil, nil
			}
			return nil, fmt.Errorf("error scanning releases row from DB. Error: %w", err)
		}

		err = json.Unmarshal(([]byte)(metadataJson), &row.Metadata)
		if err != nil {
			return nil, fmt.Errorf("error during json unmarshal. Error: %w. Data: %s", err, row.Metadata)
		}

		appLocks = append(appLocks, row)
	}

	err = closeRows(rows)
	if err != nil {
		return nil, err
	}
	return appLocks, nil
}
