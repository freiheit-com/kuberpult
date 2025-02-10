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
	"errors"
	"fmt"
	"github.com/freiheit-com/kuberpult/pkg/logger"
	"github.com/freiheit-com/kuberpult/pkg/tracing"
	"time"
)

type SyncStatus int

const (
	SYNCED = iota
	UNSYNCED
	SYNC_FAILED
)

const BULK_INSERT_BATCH_SIZE = 500

type GitSyncData struct {
	AppName       string
	EnvName       string
	TransformerID EslVersion
	SyncStatus
}

func (h *DBHandler) DBWriteNewSyncEvent(ctx context.Context, tx *sql.Tx, syncData *GitSyncData) error {
	span, ctx, onErr := tracing.StartSpanFromContext(ctx, "DBWriteNewSyncEvent")
	defer span.Finish()
	if h == nil {
		return nil
	}
	if tx == nil {
		return onErr(fmt.Errorf("DBWriteNewSyncEvent: no transaction provided"))
	}

	insertQuery := h.AdaptQuery("INSERT INTO git_sync_status (created, transformerid, envName, appName, status)  VALUES (?, ?, ?, ?, ?);")
	now, err := h.DBReadTransactionTimestamp(ctx, tx)
	if err != nil {
		return onErr(fmt.Errorf("DBWriteNewSyncEvent unable to get transaction timestamp: %w", err))
	}
	span.SetTag("query", insertQuery)
	_, err = tx.Exec(
		insertQuery,
		*now,
		syncData.TransformerID,
		syncData.EnvName,
		syncData.AppName,
		syncData.SyncStatus)

	if err != nil {
		return onErr(fmt.Errorf("could not write sync event into DB. Error: %w\n", err))
	}
	return nil
}

func (h *DBHandler) DBWriteNewSyncEventBulk(ctx context.Context, tx *sql.Tx, id TransformerID, envApps []EnvApp, status SyncStatus) error {
	span, ctx, onErr := tracing.StartSpanFromContext(ctx, "DBWriteNewSyncEventBulk")
	defer span.Finish()
	if h == nil {
		return nil
	}
	if tx == nil {
		return onErr(fmt.Errorf("DBWriteNewSyncEventBulk: no transaction provided"))
	}
	now, err := h.DBReadTransactionTimestamp(ctx, tx)
	if err != nil {
		return onErr(fmt.Errorf("DBWriteNewSyncEventBulk unable to get transaction timestamp: %w", err))
	}

	err = h.executeBulkInsert(ctx, tx, envApps, *now, id, status, BULK_INSERT_BATCH_SIZE)
	if err != nil {
		return onErr(fmt.Errorf("could not write sync event into DB. Error: %w\n", err))
	}
	return nil
}

type EnvApp struct {
	AppName string
	EnvName string
}

func (h *DBHandler) DBReadUnsyncedAppsForTransfomerID(ctx context.Context, tx *sql.Tx, id TransformerID) ([]EnvApp, error) {
	span, ctx, onErr := tracing.StartSpanFromContext(ctx, "DBReadUnsyncedAppsForTransfomerID")
	defer span.Finish()
	if h == nil {
		return nil, nil
	}
	if tx == nil {
		return nil, onErr(fmt.Errorf("DBReadUnsyncedAppsForTransfomerID: no transaction provided"))
	}

	selectQuery := h.AdaptQuery("SELECT appName, envName FROM git_sync_status WHERE transformerid = ? AND status = ? ORDER BY created DESC;")
	rows, err := tx.QueryContext(
		ctx,
		selectQuery,
		id,
		UNSYNCED,
	)
	if err != nil {
		return nil, onErr(fmt.Errorf("could not get current eslVersion. Error: %w\n", err))
	}
	defer func(rows *sql.Rows) {
		err := rows.Close()
		if err != nil {
			logger.FromContext(ctx).Sugar().Warnf("row closing error: %v", err)
		}
	}(rows)
	allCombinations := make([]EnvApp, 0)
	var currApp string
	var currEnv string
	for rows.Next() {
		err := rows.Scan(&currApp, &currEnv)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return nil, nil
			}
			return nil, onErr(fmt.Errorf("Error table for next eslVersion. Error: %w\n", err))
		}
		allCombinations = append(allCombinations, EnvApp{
			AppName: currApp,
			EnvName: currEnv,
		})
	}
	err = closeRows(rows)
	if err != nil {
		return nil, onErr(err)
	}
	return allCombinations, nil
}

func (h *DBHandler) DBReadAllAppsForTransfomerID(ctx context.Context, tx *sql.Tx, id TransformerID) ([]EnvApp, error) {
	span, ctx, onErr := tracing.StartSpanFromContext(ctx, "DBReadAllAppsForTransfomerID")
	defer span.Finish()
	if h == nil {
		return nil, nil
	}
	if tx == nil {
		return nil, onErr(fmt.Errorf("DBReadAllAppsForTransfomerID: no transaction provided"))
	}

	selectQuery := h.AdaptQuery("SELECT appName, envName FROM git_sync_status WHERE transformerid = ? ORDER BY created DESC;")
	rows, err := tx.QueryContext(
		ctx,
		selectQuery,
		id,
	)
	if err != nil {
		return nil, onErr(fmt.Errorf("could not get current eslVersion. Error: %w\n", err))
	}
	defer func(rows *sql.Rows) {
		err := rows.Close()
		if err != nil {
			logger.FromContext(ctx).Sugar().Warnf("row closing error: %v", err)
		}
	}(rows)
	allCombinations := make([]EnvApp, 0)
	var currApp string
	var currEnv string
	for rows.Next() {
		err := rows.Scan(&currApp, &currEnv)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return nil, nil
			}
			return nil, onErr(fmt.Errorf("Error table for next eslVersion. Error: %w\n", err))
		}
		allCombinations = append(allCombinations, EnvApp{
			AppName: currApp,
			EnvName: currEnv,
		})
	}
	err = closeRows(rows)
	if err != nil {
		return nil, onErr(err)
	}
	return allCombinations, nil
}

func (h *DBHandler) DBBulkUpdateUnsyncedApps(ctx context.Context, tx *sql.Tx, id TransformerID, status SyncStatus) error {
	span, ctx, onErr := tracing.StartSpanFromContext(ctx, "DBBulkUpdateUnsyncedApps")
	defer span.Finish()
	if h == nil {
		return nil
	}
	if tx == nil {
		return onErr(fmt.Errorf("DBBulkUpdateUnsyncedApps: no transaction provided"))
	}

	allCombs, err := h.DBReadUnsyncedAppsForTransfomerID(ctx, tx, id)
	if err != nil {
		return onErr(fmt.Errorf("DBBulkUpdateUnsyncedApps unable to read unsynced apps: %w", err))
	}
	if len(allCombs) == 0 {
		logger.FromContext(ctx).Sugar().Info("Could not update all unsynced apps. Did not find any unsynced apps.")
		return nil
	}
	now, err := h.DBReadTransactionTimestamp(ctx, tx)
	if err != nil {
		return onErr(fmt.Errorf("DBBulkUpdateUnsyncedApps unable to get transaction timestamp: %w", err))
	}

	return h.executeBulkInsert(ctx, tx, allCombs, *now, id, status, BULK_INSERT_BATCH_SIZE)
}

func (h *DBHandler) DBBulkUpdateAllApps(ctx context.Context, tx *sql.Tx, id TransformerID, status SyncStatus) error {
	span, ctx, onErr := tracing.StartSpanFromContext(ctx, "DBBulkUpdateAllApps")
	defer span.Finish()
	if h == nil {
		return nil
	}
	if tx == nil {
		return onErr(fmt.Errorf("DBBulkUpdateAllApps: no transaction provided"))
	}

	allCombs, err := h.DBReadAllAppsForTransfomerID(ctx, tx, id)
	if err != nil {
		return onErr(fmt.Errorf("DBBulkUpdateAllApps unable to read apps: %w", err))
	}
	if len(allCombs) == 0 {
		logger.FromContext(ctx).Sugar().Info("Could not update all apps. Did not find any unsynced apps.")
		return nil
	}
	now, err := h.DBReadTransactionTimestamp(ctx, tx)
	if err != nil {
		return onErr(fmt.Errorf("DBBulkUpdateAllApps unable to get transaction timestamp: %w", err))
	}

	return h.executeBulkInsert(ctx, tx, allCombs, *now, id, status, BULK_INSERT_BATCH_SIZE)
}

func (h *DBHandler) DBRetrieveAppsByStatus(ctx context.Context, tx *sql.Tx, status SyncStatus) ([]GitSyncData, error) {
	span, ctx, onErr := tracing.StartSpanFromContext(ctx, "DBRetrieveSyncStatus")
	defer span.Finish()
	if h == nil {
		return nil, nil
	}
	if tx == nil {
		return nil, onErr(fmt.Errorf("DBRetrieveAppsByStatus: no transaction provided"))
	}

	selectQuery := h.AdaptQuery("SELECT transformerid, envName, appName, status FROM git_sync_status WHERE status = ? ORDER BY created DESC;")
	rows, err := tx.QueryContext(
		ctx,
		selectQuery,
		status)
	result, err := processGitSyncStatusRows(ctx, rows, err)

	if err != nil {
		return nil, onErr(err)
	}
	return result, nil
}

func processGitSyncStatusRows(ctx context.Context, rows *sql.Rows, err error) ([]GitSyncData, error) {
	if err != nil {
		return nil, fmt.Errorf("could not get git sync status for apps. Error: %w\n", err)
	}

	defer func(rows *sql.Rows) {
		err := rows.Close()
		if err != nil {
			logger.FromContext(ctx).Sugar().Warnf("row closing error: %v", err)
		}
	}(rows)

	var syncData []GitSyncData
	for rows.Next() {
		//exhaustruct:ignore
		curr := GitSyncData{}
		err := rows.Scan(&curr.TransformerID, &curr.EnvName, &curr.AppName, &curr.SyncStatus)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return nil, nil
			}
			return nil, fmt.Errorf("Error table scanning git_sync_status. Error: %w\n", err)
		}
		syncData = append(syncData, curr)
	}
	err = closeRows(rows)
	if err != nil {
		return nil, err
	}
	return syncData, nil
}

func (h *DBHandler) DBRetrieveSyncStatus(ctx context.Context, tx *sql.Tx, appName, envName string) (*GitSyncData, error) {
	span, ctx, onErr := tracing.StartSpanFromContext(ctx, "DBRetrieveSyncStatus")
	defer span.Finish()
	if h == nil {
		return nil, nil
	}
	if tx == nil {
		return nil, onErr(fmt.Errorf("DBRetrieveSyncStatus: no transaction provided"))
	}

	selectQuerry := h.AdaptQuery("SELECT transformerid, envName, appName, status FROM git_sync_status WHERE appName = ? AND envName= ? ORDER BY created DESC LIMIT 1;")
	rows, err := tx.QueryContext(
		ctx,
		selectQuerry,
		appName,
		envName)

	result, err := processGitSyncStatusRows(ctx, rows, err)

	if err != nil {
		return nil, onErr(err)
	}
	if len(result) == 0 {
		return nil, nil
	}
	return &result[0], nil
}

// These queries can get long. Because of this, we insert these values in batches
func (h *DBHandler) executeBulkInsert(ctx context.Context, tx *sql.Tx, allEnvApps []EnvApp, now time.Time, id TransformerID, status SyncStatus, batchSize int) error {
	//queryTemplate := "INSERT INTO git_sync_status (created, transformerid, envName, appName, status) VALUES ;"
	queryTemplate := `INSERT INTO git_sync_status (created, transformerid, envName, appName, status)
	VALUES ('%s', %d, '%s', '%s', %d)
	ON CONFLICT(envName, appname)
	DO UPDATE SET created = excluded.created, status = excluded.status, transformerid = excluded.transformerid;`

	currentQuery := ""
	if batchSize < 1 {
		return fmt.Errorf("batch size needs to be a positive number")
	}
	for idx, currComb := range allEnvApps {
		currentQuery += fmt.Sprintf(queryTemplate, now.Format(time.RFC3339), id, currComb.EnvName, currComb.AppName, status)
		if idx%batchSize == 0 || idx == len(allEnvApps)-1 { // Is end of batch || tail
			_, err := tx.ExecContext(ctx, currentQuery)
			if err != nil {
				return err
			}
			currentQuery = ""
		}
	}
	return nil
}
