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
	"gopkg.in/DataDog/dd-trace-go.v1/ddtrace/tracer"
	"time"
)

type TeamLock struct {
	Created  time.Time
	LockID   string
	Env      string
	Team     string
	Metadata LockMetadata
}

type TeamLockHistory struct {
	Created  time.Time
	LockID   string
	Env      string
	Team     string
	Metadata LockMetadata
	Deleted  bool
}

// SELECTS

func (h *DBHandler) DBSelectAllTeamLocksOfAllEnvs(ctx context.Context, tx *sql.Tx) (map[string]map[string][]TeamLock, error) {
	span, ctx := tracer.StartSpanFromContext(ctx, "DBSelectAllTeamLocksOfAllEnvs")
	defer span.Finish()
	if h == nil {
		return nil, nil
	}
	if tx == nil {
		return nil, fmt.Errorf("DBSelectAllTeamLocksOfAllEnvs: no transaction provided")
	}

	selectQuery := h.AdaptQuery(`
		SELECT created, lockID, envName, teamName, metadata
		FROM team_locks;`)
	span.SetTag("query", selectQuery)

	rows, err := tx.QueryContext(
		ctx,
		selectQuery,
	)
	if err != nil {
		return nil, fmt.Errorf("could not read team lock from DB. Error: %w\n", err)
	}
	teamLocks := make(map[string]map[string][]TeamLock)
	for rows.Next() {
		var row = TeamLock{
			Created: time.Time{},
			LockID:  "",
			Env:     "",
			Team:    "",
			Metadata: LockMetadata{
				CreatedByName:  "",
				CreatedByEmail: "",
				Message:        "",
				CiLink:         "",
				CreatedAt:      time.Time{},
			},
		}
		var metadata string

		err := rows.Scan(&row.Created, &row.LockID, &row.Env, &row.Team, &metadata)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return nil, nil
			}
			return nil, fmt.Errorf("Error scanning environment locks row from DB. Error: %w\n", err)
		}

		//exhaustruct:ignore
		var resultJson = LockMetadata{}
		err = json.Unmarshal(([]byte)(metadata), &resultJson)
		if err != nil {
			return nil, fmt.Errorf("Error during json unmarshal. Error: %w. Data: %s\n", err, row.Metadata)
		}
		if _, ok := teamLocks[row.Env]; !ok {
			teamLocks[row.Env] = make(map[string][]TeamLock)
		}
		if _, ok := teamLocks[row.Env][row.Team]; !ok {
			teamLocks[row.Env][row.Team] = make([]TeamLock, 0)
		}
		teamLocks[row.Env][row.Team] = append(teamLocks[row.Env][row.Team], TeamLock{
			Created:  row.Created,
			LockID:   row.LockID,
			Env:      row.Env,
			Team:     row.Team,
			Metadata: resultJson,
		})
	}
	err = closeRows(rows)
	if err != nil {
		return nil, err
	}

	return teamLocks, nil
}

func (h *DBHandler) DBSelectAnyActiveTeamLock(ctx context.Context, tx *sql.Tx) (*TeamLock, error) {
	span, ctx := tracer.StartSpanFromContext(ctx, "DBSelectAnyActiveTeamLock")
	defer span.Finish()

	selectQuery := h.AdaptQuery(`
		SELECT created, lockid, environment, teamName, metadata 
		FROM team_locks 
		LIMIT 1;`)
	span.SetTag("query", selectQuery)

	rows, err := tx.QueryContext(
		ctx,
		selectQuery,
	)
	if err != nil {
		return nil, fmt.Errorf("could not read team lock from DB. Error: %w\n", err)
	}
	if rows.Next() {
		var row = TeamLock{
			Created: time.Time{},
			LockID:  "",
			Env:     "",
			Team:    "",
			Metadata: LockMetadata{
				CreatedByName:  "",
				CreatedByEmail: "",
				Message:        "",
				CiLink:         "",
				CreatedAt:      time.Time{},
			},
		}
		var metadata string

		err := rows.Scan(&row.Created, &row.LockID, &row.Env, &row.Team, &metadata)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return nil, nil
			}
			return nil, fmt.Errorf("Error scanning team locks row from DB. Error: %w\n", err)
		}

		//exhaustruct:ignore
		var resultJson = LockMetadata{}
		err = json.Unmarshal(([]byte)(metadata), &resultJson)
		if err != nil {
			return nil, fmt.Errorf("Error during json unmarshal. Error: %w. Data: %s\n", err, row.Metadata)
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

func (h *DBHandler) DBSelectTeamLocksForEnv(ctx context.Context, tx *sql.Tx, environment string) ([]TeamLock, error) {
	span, ctx := tracer.StartSpanFromContext(ctx, "DBSelectTeamLocksForEnv")
	defer span.Finish()

	selectQuery := h.AdaptQuery(`
		SELECT created, lockid, envname, teamname, metadata
		FROM team_locks 
		WHERE envname = (?)
		ORDER BY lockid;`)
	span.SetTag("query", selectQuery)

	rows, err := tx.QueryContext(
		ctx,
		selectQuery,
		environment,
	)
	return h.processTeamLockRows(ctx, err, rows)
}

func (h *DBHandler) DBSelectAllActiveTeamLocksForTeam(ctx context.Context, tx *sql.Tx, teamName string) ([]TeamLock, error) {
	span, ctx := tracer.StartSpanFromContext(ctx, "DBSelectAllActiveTeamLocksForApp")
	defer span.Finish()

	if h == nil {
		return nil, nil
	}
	if tx == nil {
		return nil, fmt.Errorf("DBSelectAllActiveTeamLocksForTeam: no transaction provided")
	}

	selectQuery := h.AdaptQuery(`
		SELECT created, lockid, envName, teamName, metadata
		FROM team_locks
		WHERE team_locks.teamName = (?);`)
	rows, err := tx.QueryContext(ctx, selectQuery, teamName)
	return h.processTeamLockRows(ctx, err, rows)
}

func (h *DBHandler) DBSelectTeamLock(ctx context.Context, tx *sql.Tx, environment, teamName, lockID string) (*TeamLock, error) {
	span, ctx := tracer.StartSpanFromContext(ctx, "DBSelectTeamLock")
	defer span.Finish()

	selectQuery := h.AdaptQuery(`
		SELECT created, lockID, envName, teamName, metadata
		FROM team_locks_history
		WHERE envName=? AND teamName=? AND lockID=?
		ORDER BY version DESC
		LIMIT 1;`)
	span.SetTag("query", selectQuery)

	rows, err := tx.QueryContext(
		ctx,
		selectQuery,
		environment,
		teamName,
		lockID,
	)
	result, err := h.processTeamLockRows(ctx, err, rows)
	if err != nil {
		return nil, err
	}
	if len(result) == 0 {
		return nil, nil
	}
	return &result[0], nil
}

func (h *DBHandler) DBSelectAllTeamLocks(ctx context.Context, tx *sql.Tx, environment, teamName string) ([]string, error) {
	span, ctx := tracer.StartSpanFromContext(ctx, "DBSelectAllTeamLocks")
	defer span.Finish()
	if h == nil {
		return nil, nil
	}
	if tx == nil {
		return nil, fmt.Errorf("DBSelectAllTeamLocks: no transaction provided")
	}
	selectQuery := h.AdaptQuery(`
		SELECT lockid 
		FROM team_locks 
		WHERE envname = ? AND teamName = ? 
		ORDER BY lockid;`)
	span.SetTag("query", selectQuery)

	rows, err := tx.QueryContext(ctx, selectQuery, environment, teamName)
	return h.processAllTeamLocksRows(ctx, err, rows)
}

// DBSelectTeamLockHistory returns the last N events associated with some lock on some environment for some team. Currently only used in testing.
func (h *DBHandler) DBSelectTeamLockHistory(ctx context.Context, tx *sql.Tx, environmentName, teamName, lockID string, limit int) ([]TeamLockHistory, error) {
	span, ctx := tracer.StartSpanFromContext(ctx, "DBSelectTeamLockHistory")
	defer span.Finish()

	if h == nil {
		return nil, nil
	}
	if tx == nil {
		return nil, fmt.Errorf("DBSelectTeamLockHistory: no transaction provided")
	}

	selectQuery := h.AdaptQuery(
		fmt.Sprintf(
			"SELECT created, lockID, envName, teamName, metadata, deleted" +
				" FROM team_locks_history " +
				" WHERE envName=? AND lockID=? AND teamName=?" +
				" ORDER BY version DESC " +
				" LIMIT ?;"))

	span.SetTag("query", selectQuery)
	rows, err := tx.QueryContext(
		ctx,
		selectQuery,
		environmentName,
		lockID,
		teamName,
		limit,
	)
	if err != nil {
		return nil, fmt.Errorf("could not read team lock from DB. Error: %w\n", err)
	}
	defer func(rows *sql.Rows) {
		err := rows.Close()
		if err != nil {
			logger.FromContext(ctx).Sugar().Warnf("team locks: row could not be closed: %v", err)
		}
	}(rows)
	teamLocks := make([]TeamLockHistory, 0)
	for rows.Next() {
		var row = TeamLockHistory{
			Created: time.Time{},
			LockID:  "",
			Team:    "",
			Env:     "",
			Deleted: true,
			Metadata: LockMetadata{
				CiLink:         "",
				CreatedByName:  "",
				CreatedByEmail: "",
				CreatedAt:      time.Time{},
				Message:        "",
			},
		}
		var metadata string

		err := rows.Scan(&row.Created, &row.LockID, &row.Env, &row.Team, &metadata, &row.Deleted)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return nil, nil
			}
			return nil, fmt.Errorf("Error scanning team locks row from DB. Error: %w\n", err)
		}

		//exhaustruct:ignore
		var resultJson = LockMetadata{}
		err = json.Unmarshal(([]byte)(metadata), &resultJson)
		if err != nil {
			return nil, fmt.Errorf("Error during json unmarshal. Error: %w. Data: %s\n", err, row.Metadata)
		}
		row.Metadata = resultJson
		teamLocks = append(teamLocks, row)
	}
	err = closeRows(rows)
	if err != nil {
		return nil, err
	}
	return teamLocks, nil
}

// INSERT, UPDATE, DELETES
func (h *DBHandler) DBWriteTeamLock(ctx context.Context, tx *sql.Tx, lockID, environment, teamName string, metadata LockMetadata) error {
	span, ctx := tracer.StartSpanFromContext(ctx, "DBWriteTeamLock")
	defer span.Finish()

	if h == nil {
		return nil
	}
	if tx == nil {
		return fmt.Errorf("DBWriteTeamLock: no transaction provided")
	}
	err := h.upsertTeamLockRow(ctx, tx, lockID, environment, teamName, metadata)
	if err != nil {
		return err
	}
	err = h.insertTeamLockHistoryRow(ctx, tx, lockID, environment, teamName, metadata, false)
	if err != nil {
		return err
	}
	return nil
}

func (h *DBHandler) DBSelectTeamLockSet(ctx context.Context, tx *sql.Tx, environment, teamName string, lockIDs []string) ([]TeamLock, error) {
	span, ctx := tracer.StartSpanFromContext(ctx, "DBSelectTeamLockSet")
	defer span.Finish()

	if len(lockIDs) == 0 {
		return nil, nil
	}
	if h == nil {
		return nil, nil
	}
	if tx == nil {
		return nil, fmt.Errorf("DBSelectTeamLockSet: no transaction provided")
	}

	var teamLocks []TeamLock
	//Get the latest change to each lock
	for _, id := range lockIDs {
		teamLock, err := h.DBSelectTeamLock(ctx, tx, environment, teamName, id)
		if err != nil {
			return nil, err
		}
		teamLocks = append(teamLocks, *teamLock)
	}
	return teamLocks, nil
}

func (h *DBHandler) DBDeleteTeamLock(ctx context.Context, tx *sql.Tx, environment, teamName, lockID string) error {
	span, ctx := tracer.StartSpanFromContext(ctx, "DBDeleteTeamLock")
	defer span.Finish()

	if h == nil {
		return nil
	}
	if tx == nil {
		return fmt.Errorf("DBDeleteTeamLock: no transaction provided")
	}
	existingTeamLock, err := h.DBSelectTeamLock(ctx, tx, environment, teamName, lockID)

	if err != nil {
		return fmt.Errorf("Could not obtain existing team lock: %w\n", err)
	}

	if existingTeamLock == nil {
		logger.FromContext(ctx).Sugar().Warnf("could not delete team lock. The team lock '%s' on team '%s' on environment '%s' does not exist. Continuing anyway.", lockID, teamName, environment)
		return nil
	}
	err = h.deleteTeamLockRow(ctx, tx, lockID, environment, teamName)
	if err != nil {
		return err
	}
	err = h.insertTeamLockHistoryRow(ctx, tx, lockID, environment, teamName, existingTeamLock.Metadata, true)
	if err != nil {
		return err
	}
	return nil
}

// actual changes in tables

func (h *DBHandler) upsertTeamLockRow(ctx context.Context, transaction *sql.Tx, lockID, environment, teamName string, metadata LockMetadata) error {
	span, ctx := tracer.StartSpanFromContext(ctx, "upsertTeamLockRow")
	defer span.Finish()
	upsertQuery := h.AdaptQuery(`
		INSERT INTO team_locks (created, lockId, envname, teamName, metadata)
		VALUES (?, ?, ?, ?, ?)
		ON CONFLICT(teamname, envname, lockid)
		DO UPDATE SET created = excluded.created, lockid = excluded.lockid, metadata = excluded.metadata, envname = excluded.envname, teamname = excluded.teamname;
	`)
	span.SetTag("query", upsertQuery)
	jsonToInsert, err := json.Marshal(metadata)
	if err != nil {
		return fmt.Errorf("could not marshal json data: %w", err)
	}

	now, err := h.DBReadTransactionTimestamp(ctx, transaction)
	if err != nil {
		return fmt.Errorf("upsertTeamLockRow unable to get transaction timestamp: %w", err)
	}
	_, err = transaction.Exec(
		upsertQuery,
		*now,
		lockID,
		environment,
		teamName,
		jsonToInsert,
	)
	if err != nil {
		return fmt.Errorf(
			"could not insert team lock for team '%s' and environment '%s' and lockid '%s' into DB. Error: %w\n",
			teamName,
			environment,
			lockID,
			err)
	}
	return nil
}

func (h *DBHandler) deleteTeamLockRow(ctx context.Context, transaction *sql.Tx, lockId, environment, teamName string) error {
	span, _ := tracer.StartSpanFromContext(ctx, "deleteTeamLockRow")
	defer span.Finish()
	deleteQuery := h.AdaptQuery(`
		DELETE FROM team_locks
		WHERE teamname=? AND lockId=? AND envname=?;`)
	span.SetTag("query", deleteQuery)
	_, err := transaction.Exec(
		deleteQuery,
		teamName,
		lockId,
		environment,
	)
	if err != nil {
		return fmt.Errorf(
			"could not delete team_lock for team '%s' and environment '%s' and lockId '%s' from DB. Error: %w\n",
			teamName,
			environment,
			lockId,
			err)
	}
	return nil
}

func (h *DBHandler) insertTeamLockHistoryRow(ctx context.Context, transaction *sql.Tx, lockID, environment, teamName string, metadata LockMetadata, deleted bool) error {
	span, ctx := tracer.StartSpanFromContext(ctx, "insertTeamLockHistoryRow")
	defer span.Finish()
	upsertQuery := h.AdaptQuery(`
		INSERT INTO team_locks_history (created, lockId, envname, teamName, metadata, deleted)
		VALUES (?, ?, ?, ?, ?, ?);
	`)
	span.SetTag("query", upsertQuery)
	jsonToInsert, err := json.Marshal(metadata)
	if err != nil {
		return fmt.Errorf("could not marshal json data: %w", err)
	}

	now, err := h.DBReadTransactionTimestamp(ctx, transaction)
	if err != nil {
		return fmt.Errorf("upsertTeamLockRow unable to get transaction timestamp: %w", err)
	}
	_, err = transaction.Exec(
		upsertQuery,
		*now,
		lockID,
		environment,
		teamName,
		jsonToInsert,
		deleted,
	)
	if err != nil {
		return fmt.Errorf(
			"could not insert team lock history for team '%s' and environment '%s' and lockid '%s' into DB. Error: %w\n",
			teamName,
			environment,
			lockID,
			err)
	}
	return nil
}

// process rows functions
func (h *DBHandler) processTeamLockRows(ctx context.Context, err error, rows *sql.Rows) ([]TeamLock, error) {
	if err != nil {
		return nil, fmt.Errorf("could not read team lock from DB. Error: %w\n", err)
	}
	defer func(rows *sql.Rows) {
		err := rows.Close()
		if err != nil {
			logger.FromContext(ctx).Sugar().Warnf("releases: row could not be closed: %v", err)
		}
	}(rows)
	teamLocks := make([]TeamLock, 0)
	for rows.Next() {
		var row = TeamLock{
			Created: time.Time{},
			LockID:  "",
			Env:     "",
			Team:    "",
			Metadata: LockMetadata{
				CreatedByName:  "",
				CreatedByEmail: "",
				Message:        "",
				CiLink:         "",
				CreatedAt:      time.Time{},
			},
		}
		var metadata string

		err := rows.Scan(&row.Created, &row.LockID, &row.Env, &row.Team, &metadata)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return nil, nil
			}
			return nil, fmt.Errorf("Error scanning environment locks row from DB. Error: %w\n", err)
		}

		//exhaustruct:ignore
		var resultJson = LockMetadata{}
		err = json.Unmarshal(([]byte)(metadata), &resultJson)
		if err != nil {
			return nil, fmt.Errorf("Error during json unmarshal. Error: %w. Data: %s\n", err, row.Metadata)
		}
		row.Metadata = resultJson
		teamLocks = append(teamLocks, row)
	}
	err = closeRows(rows)
	if err != nil {
		return nil, err
	}

	return teamLocks, nil
}

func (h *DBHandler) processAllTeamLocksRows(ctx context.Context, err error, rows *sql.Rows) ([]string, error) {
	span, ctx := tracer.StartSpanFromContext(ctx, "processAllTeamLocksRow")
	defer span.Finish()

	if err != nil {
		return nil, fmt.Errorf("could not query all_team_locks table from DB. Error: %w\n", err)
	}
	defer func(rows *sql.Rows) {
		err := rows.Close()
		if err != nil {
			logger.FromContext(ctx).Sugar().Warnf("releases: row could not be closed: %v", err)
		}
	}(rows)
	//exhaustruct:ignore
	var result []string = make([]string, 0)
	for rows.Next() {
		var lockId string
		err := rows.Scan(&lockId)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return nil, nil
			}
			return nil, fmt.Errorf("Error scanning releases row from DB. Error: %w\n", err)
		}

		result = append(result, lockId)
	}
	err = closeRows(rows)
	if err != nil {
		return nil, err
	}
	return result, nil
}
