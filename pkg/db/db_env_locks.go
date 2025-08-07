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
	"github.com/freiheit-com/kuberpult/pkg/logger"
	"github.com/freiheit-com/kuberpult/pkg/types"
	"gopkg.in/DataDog/dd-trace-go.v1/ddtrace/tracer"
	"time"
)

type EnvironmentLock struct {
	Created  time.Time
	LockID   string
	Env      types.EnvName
	Metadata LockMetadata
}

type EnvLockHistory struct {
	Created  time.Time
	LockID   string
	Env      types.EnvName
	Metadata LockMetadata
	Deleted  bool
}

// SELECTS
func (h *DBHandler) DBSelectAllEnvLocksOfAllEnvs(ctx context.Context, tx *sql.Tx) (map[types.EnvName][]EnvironmentLock, error) {
	span, ctx := tracer.StartSpanFromContext(ctx, "DBSelectAllEnvLocksOfAllEnvs")
	defer span.Finish()
	if h == nil {
		return nil, nil
	}
	if tx == nil {
		return nil, fmt.Errorf("DBSelectAllEnvLocksOfAllEnvs: no transaction provided")
	}

	selectQuery := h.AdaptQuery(`
		SELECT created, lockID, envName, metadata
		FROM environment_locks;`)
	span.SetTag("query", selectQuery)

	rows, err := tx.QueryContext(
		ctx,
		selectQuery,
	)
	if err != nil {
		return nil, fmt.Errorf("could not read environment lock from DB. Error: %w", err)
	}
	envLocks := make(map[types.EnvName][]EnvironmentLock)
	for rows.Next() {
		var row = EnvironmentLock{
			Created: time.Time{},
			LockID:  "",
			Env:     "",
			Metadata: LockMetadata{
				CreatedByName:     "",
				CreatedByEmail:    "",
				Message:           "",
				CiLink:            "",
				CreatedAt:         time.Time{},
				SuggestedLifeTime: "",
			},
		}
		var metadata string

		err := rows.Scan(&row.Created, &row.LockID, &row.Env, &metadata)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return nil, nil
			}
			return nil, fmt.Errorf("error scanning environment locks row from DB. Error: %w", err)
		}

		//exhaustruct:ignore
		var resultJson = LockMetadata{}
		err = json.Unmarshal(([]byte)(metadata), &resultJson)
		if err != nil {
			return nil, fmt.Errorf("error during json unmarshal. Error: %w. Data: %s", err, row.Metadata)
		}
		if _, ok := envLocks[row.Env]; !ok {
			envLocks[row.Env] = make([]EnvironmentLock, 0)
		}

		envLocks[row.Env] = append(envLocks[row.Env], EnvironmentLock{
			Created:  row.Created,
			LockID:   row.LockID,
			Env:      row.Env,
			Metadata: resultJson,
		})
	}
	err = closeRows(rows)
	if err != nil {
		return nil, err
	}

	return envLocks, nil
}

func (h *DBHandler) DBSelectAnyActiveEnvLock(ctx context.Context, tx *sql.Tx) (*EnvironmentLock, error) {
	span, ctx := tracer.StartSpanFromContext(ctx, "DBSelectAnyActiveEnvLock")
	defer span.Finish()

	selectQuery := h.AdaptQuery(`
		SELECT created, lockid, envname, metadata 
		FROM environment_locks 
		LIMIT 1;`)
	span.SetTag("query", selectQuery)

	rows, err := tx.QueryContext(
		ctx,
		selectQuery,
	)
	if err != nil {
		return nil, fmt.Errorf("could not read environment lock from DB. Error: %w", err)
	}
	if rows.Next() {
		var row = EnvironmentLock{
			Created: time.Time{},
			LockID:  "",
			Env:     "",
			Metadata: LockMetadata{
				CreatedByName:     "",
				CreatedByEmail:    "",
				Message:           "",
				CiLink:            "",
				CreatedAt:         time.Time{},
				SuggestedLifeTime: "",
			},
		}
		var metadata string

		err := rows.Scan(&row.Created, &row.LockID, &row.Env, &metadata)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return nil, nil
			}
			return nil, fmt.Errorf("error scanning environment locks row from DB. Error: %w", err)
		}

		//exhaustruct:ignore
		var resultJson = LockMetadata{}
		err = json.Unmarshal(([]byte)(metadata), &resultJson)
		if err != nil {
			return nil, fmt.Errorf("error during json unmarshal. Error: %w. Data: %s", err, row.Metadata)
		}
		row.Metadata = resultJson
		return &row, nil
	}
	err = closeRows(rows)
	if err != nil {
		return nil, err
	}
	return nil, nil
}

func (h *DBHandler) DBSelectEnvLocksForEnv(ctx context.Context, tx *sql.Tx, environment types.EnvName) ([]EnvironmentLock, error) {
	span, ctx := tracer.StartSpanFromContext(ctx, "DBSelectEnvLocksForEnv")
	defer span.Finish()

	selectQuery := h.AdaptQuery(`
		SELECT created, lockid, envname, metadata
		FROM environment_locks 
		WHERE envname = (?)
		ORDER BY lockid;`)
	span.SetTag("query", selectQuery)

	rows, err := tx.QueryContext(
		ctx,
		selectQuery,
		environment,
	)
	return h.processEnvLockRows(ctx, err, rows)
}

func (h *DBHandler) DBSelectAllActiveEnvLocks(ctx context.Context, tx *sql.Tx, envName string) ([]EnvironmentLock, error) {
	span, ctx := tracer.StartSpanFromContext(ctx, "DBSelectAllActiveEnvLocks")
	defer span.Finish()

	if h == nil {
		return nil, nil
	}
	if tx == nil {
		return nil, fmt.Errorf("DBSelectAllActiveEnvLocks: no transaction provided")
	}

	selectQuery := h.AdaptQuery(`
		SELECT created, lockid, envName, metadata
		FROM environment_locks
		WHERE envName = (?);`)
	rows, err := tx.QueryContext(ctx, selectQuery, envName)
	return h.processEnvLockRows(ctx, err, rows)
}

func (h *DBHandler) DBSelectEnvLock(ctx context.Context, tx *sql.Tx, environment types.EnvName, lockID string) (*EnvironmentLock, error) {
	span, ctx := tracer.StartSpanFromContext(ctx, "DBSelectEnvLock")
	defer span.Finish()

	selectQuery := h.AdaptQuery(`
		SELECT created, lockID, envName, metadata
		FROM environment_locks_history
		WHERE envName=? AND lockID=?
		ORDER BY created DESC
		LIMIT 1;`)
	span.SetTag("query", selectQuery)

	rows, err := tx.QueryContext(
		ctx,
		selectQuery,
		environment,
		lockID,
	)
	result, err := h.processEnvLockRows(ctx, err, rows)
	if err != nil {
		return nil, err
	}
	if len(result) == 0 {
		return nil, nil
	}
	return &result[0], nil
}

func (h *DBHandler) DBSelectAllEnvLocks(ctx context.Context, tx *sql.Tx, environment types.EnvName) ([]string, error) {
	span, ctx := tracer.StartSpanFromContext(ctx, "DBSelectAllEnvLocks")
	defer span.Finish()
	if h == nil {
		return nil, nil
	}
	if tx == nil {
		return nil, fmt.Errorf("DBSelectAllEnvLocks: no transaction provided")
	}
	selectQuery := h.AdaptQuery(`
		SELECT lockid 
		FROM environment_locks 
		WHERE envname = ?
		ORDER BY lockid;`)
	span.SetTag("query", selectQuery)

	rows, err := tx.QueryContext(ctx, selectQuery, environment)
	return h.processAllEnvLocksRows(ctx, err, rows)
}

// DBSelectEnvLockHistory returns the last N events associated with some lock on some environment for some environment. Currently only used in testing.
func (h *DBHandler) DBSelectEnvLockHistory(ctx context.Context, tx *sql.Tx, environmentName types.EnvName, lockID string, limit int) ([]EnvLockHistory, error) {
	span, ctx := tracer.StartSpanFromContext(ctx, "DBSelectEnvLockHistory")
	defer span.Finish()

	if h == nil {
		return nil, nil
	}
	if tx == nil {
		return nil, fmt.Errorf("DBSelectEnvLockHistory: no transaction provided")
	}

	selectQuery := h.AdaptQuery(
		fmt.Sprintf(
			"SELECT created, lockID, envName, metadata, deleted" +
				" FROM environment_locks_history " +
				" WHERE envName=? AND lockID=?" +
				" ORDER BY version DESC " +
				" LIMIT ?;"))

	span.SetTag("query", selectQuery)
	rows, err := tx.QueryContext(
		ctx,
		selectQuery,
		environmentName,
		lockID,
		limit,
	)
	if err != nil {
		return nil, fmt.Errorf("could not read environment lock from DB. Error: %w", err)
	}
	defer func(rows *sql.Rows) {
		err := rows.Close()
		if err != nil {
			logger.FromContext(ctx).Sugar().Warnf("environment locks: row could not be closed: %v", err)
		}
	}(rows)
	envLocks := make([]EnvLockHistory, 0)
	for rows.Next() {
		var row = EnvLockHistory{
			Created: time.Time{},
			LockID:  "",
			Env:     "",
			Deleted: true,
			Metadata: LockMetadata{
				CiLink:            "",
				CreatedByName:     "",
				CreatedByEmail:    "",
				CreatedAt:         time.Time{},
				Message:           "",
				SuggestedLifeTime: "",
			},
		}
		var metadata string

		err := rows.Scan(&row.Created, &row.LockID, &row.Env, &metadata, &row.Deleted)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return nil, nil
			}
			return nil, fmt.Errorf("error scanning environment locks row from DB. Error: %w", err)
		}

		//exhaustruct:ignore
		var resultJson = LockMetadata{}
		err = json.Unmarshal(([]byte)(metadata), &resultJson)
		if err != nil {
			return nil, fmt.Errorf("error during json unmarshal. Error: %w. Data: %s", err, row.Metadata)
		}
		row.Metadata = resultJson
		envLocks = append(envLocks, row)
	}
	err = closeRows(rows)
	if err != nil {
		return nil, err
	}
	return envLocks, nil
}

// INSERT, UPDATE, DELETES
func (h *DBHandler) DBWriteEnvironmentLock(ctx context.Context, tx *sql.Tx, lockID string, environment types.EnvName, metadata LockMetadata) error {
	span, ctx := tracer.StartSpanFromContext(ctx, "DBWriteEnvironmentLock")
	defer span.Finish()

	if h == nil {
		return nil
	}
	if tx == nil {
		return fmt.Errorf("DBWriteEnvironmentLock: no transaction provided")
	}
	err := h.upsertEnvLockRow(ctx, tx, lockID, environment, metadata)
	if err != nil {
		return err
	}
	err = h.insertEnvLockHistoryRow(ctx, tx, lockID, environment, metadata, false)
	if err != nil {
		return err
	}
	return nil
}

func (h *DBHandler) DBSelectEnvLockSet(ctx context.Context, tx *sql.Tx, environment types.EnvName, lockIDs []string) ([]EnvironmentLock, error) {
	span, ctx := tracer.StartSpanFromContext(ctx, "DBSelectEnvLockSet")
	defer span.Finish()

	if len(lockIDs) == 0 {
		return nil, nil
	}
	if h == nil {
		return nil, nil
	}
	if tx == nil {
		return nil, fmt.Errorf("DBSelectEnvLockSet: no transaction provided")
	}

	var envLocks []EnvironmentLock
	//Get the latest change to each lock
	for _, id := range lockIDs {
		envLock, err := h.DBSelectEnvLock(ctx, tx, environment, id)
		if err != nil {
			return nil, err
		}
		envLocks = append(envLocks, *envLock)
	}
	return envLocks, nil
}

func (h *DBHandler) DBDeleteEnvironmentLock(ctx context.Context, tx *sql.Tx, environment types.EnvName, lockID string) error {
	span, ctx := tracer.StartSpanFromContext(ctx, "DBDeleteEnvironmentLock")
	defer span.Finish()

	if h == nil {
		return nil
	}
	if tx == nil {
		return fmt.Errorf("DBDeleteEnvironmentLock: no transaction provided")
	}
	existingEnvLock, err := h.DBSelectEnvLock(ctx, tx, environment, lockID)

	if err != nil {
		return fmt.Errorf("could not obtain existing environment lock: %w", err)
	}

	if existingEnvLock == nil {
		logger.FromContext(ctx).Sugar().Warnf("could not delete enviroment lock. The enviroment lock '%s' on enviroment '%s' does not exist. Continuing anyway.", lockID, environment)
		return nil
	}
	err = h.deleteEnvLockRow(ctx, tx, lockID, environment)
	if err != nil {
		return err
	}
	err = h.insertEnvLockHistoryRow(ctx, tx, lockID, environment, existingEnvLock.Metadata, true)
	if err != nil {
		return err
	}
	return nil
}

// actual changes in tables

func (h *DBHandler) upsertEnvLockRow(ctx context.Context, transaction *sql.Tx, lockID string, environment types.EnvName, metadata LockMetadata) error {
	span, ctx := tracer.StartSpanFromContext(ctx, "upsertEnvLockRow")
	defer span.Finish()
	upsertQuery := h.AdaptQuery(`
		INSERT INTO environment_locks (created, lockId, envname, metadata)
		VALUES (?, ?, ?, ?)
		ON CONFLICT(envname, lockid)
		DO UPDATE SET created = excluded.created, lockid = excluded.lockid, metadata = excluded.metadata, envname = excluded.envname;
	`)
	span.SetTag("query", upsertQuery)
	jsonToInsert, err := json.Marshal(metadata)
	if err != nil {
		return fmt.Errorf("could not marshal json data: %w", err)
	}

	now, err := h.DBReadTransactionTimestamp(ctx, transaction)
	if err != nil {
		return fmt.Errorf("upsertEnvLockRow unable to get transaction timestamp: %w", err)
	}
	_, err = transaction.Exec(
		upsertQuery,
		*now,
		lockID,
		environment,
		jsonToInsert,
	)
	if err != nil {
		return fmt.Errorf(
			"could not insert environment lock for '%s' and lockid '%s' into DB. Error: %w",
			environment,
			lockID,
			err)
	}
	return nil
}

func (h *DBHandler) deleteEnvLockRow(ctx context.Context, transaction *sql.Tx, lockId string, environment types.EnvName) error {
	span, _ := tracer.StartSpanFromContext(ctx, "deleteEnvLockRow")
	defer span.Finish()
	deleteQuery := h.AdaptQuery(`
		DELETE FROM environment_locks
		WHERE lockId=? AND envname=?;`)
	span.SetTag("query", deleteQuery)
	_, err := transaction.Exec(
		deleteQuery,
		lockId,
		environment,
	)
	if err != nil {
		return fmt.Errorf(
			"could not delete environment lock for '%s' and lockId '%s' from DB. Error: %w",
			environment,
			lockId,
			err)
	}
	return nil
}

func (h *DBHandler) insertEnvLockHistoryRow(ctx context.Context, transaction *sql.Tx, lockID string, environment types.EnvName, metadata LockMetadata, deleted bool) error {
	span, ctx := tracer.StartSpanFromContext(ctx, "insertEnvLockHistoryRow")
	defer span.Finish()
	upsertQuery := h.AdaptQuery(`
		INSERT INTO environment_locks_history (created, lockId, envname, metadata, deleted)
		VALUES (?, ?, ?, ?, ?);
	`)
	span.SetTag("query", upsertQuery)
	jsonToInsert, err := json.Marshal(metadata)
	if err != nil {
		return fmt.Errorf("could not marshal json data: %w", err)
	}

	now, err := h.DBReadTransactionTimestamp(ctx, transaction)
	if err != nil {
		return fmt.Errorf("insertEnvLockHistoryRow unable to get transaction timestamp: %w", err)
	}
	_, err = transaction.Exec(
		upsertQuery,
		*now,
		lockID,
		environment,
		jsonToInsert,
		deleted,
	)
	if err != nil {
		return fmt.Errorf(
			"could not insert environment lock history '%s' and lockid '%s' into DB. Error: %w",
			environment,
			lockID,
			err)
	}
	return nil
}

// process rows functions
func (h *DBHandler) processEnvLockRows(ctx context.Context, err error, rows *sql.Rows) ([]EnvironmentLock, error) {
	if err != nil {
		return nil, fmt.Errorf("could not read environment lock from DB. Error: %w", err)
	}
	defer func(rows *sql.Rows) {
		err := rows.Close()
		if err != nil {
			logger.FromContext(ctx).Sugar().Warnf("environment locks: row could not be closed: %v", err)
		}
	}(rows)
	envLocks := make([]EnvironmentLock, 0)
	for rows.Next() {
		var row = EnvironmentLock{
			Created: time.Time{},
			LockID:  "",
			Env:     "",
			Metadata: LockMetadata{
				CreatedByName:     "",
				CreatedByEmail:    "",
				Message:           "",
				CiLink:            "",
				CreatedAt:         time.Time{},
				SuggestedLifeTime: "",
			},
		}
		var metadata string

		err := rows.Scan(&row.Created, &row.LockID, &row.Env, &metadata)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return nil, nil
			}
			return nil, fmt.Errorf("error scanning environment locks row from DB. Error: %w", err)
		}

		//exhaustruct:ignore
		var resultJson = LockMetadata{}
		err = json.Unmarshal(([]byte)(metadata), &resultJson)
		if err != nil {
			return nil, fmt.Errorf("error during json unmarshal. Error: %w. Data: %s", err, row.Metadata)
		}
		row.Metadata = resultJson
		envLocks = append(envLocks, row)
	}
	err = closeRows(rows)
	if err != nil {
		return nil, err
	}

	return envLocks, nil
}

func (h *DBHandler) processAllEnvLocksRows(ctx context.Context, err error, rows *sql.Rows) ([]string, error) {
	span, ctx := tracer.StartSpanFromContext(ctx, "processAllEnvLocksRows")
	defer span.Finish()

	if err != nil {
		return nil, fmt.Errorf("could not query all environment locks table from DB. Error: %w", err)
	}
	defer func(rows *sql.Rows) {
		err := rows.Close()
		if err != nil {
			logger.FromContext(ctx).Sugar().Warnf("environment locks: row could not be closed: %v", err)
		}
	}(rows)
	//exhaustruct:ignore
	var result = make([]string, 0)
	for rows.Next() {
		var lockId string
		err := rows.Scan(&lockId)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return nil, nil
			}
			return nil, fmt.Errorf("error scanning releases row from DB. Error: %w", err)
		}

		result = append(result, lockId)
	}
	err = closeRows(rows)
	if err != nil {
		return nil, err
	}
	return result, nil
}
