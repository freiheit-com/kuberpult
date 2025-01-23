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
	"gopkg.in/DataDog/dd-trace-go.v1/ddtrace/tracer"
	"time"
)

type SyncStatus int

const (
	SYNCED = iota
	UNSYNCED
	SYNC_FAILED
)

const BULK_INSERT_BATCH_SIZE = 500 //500 is an arbi

type GitSyncData struct {
	AppName       string
	EnvName       string
	TransformerID EslVersion
	SyncStatus
}

func (h *DBHandler) DBWriteNewSyncEvent(ctx context.Context, tx *sql.Tx, syncData *GitSyncData) error {
	span, ctx := tracer.StartSpanFromContext(ctx, "DBWriteNewSyncEvent")
	defer span.Finish()
	if h == nil {
		return nil
	}
	if tx == nil {
		return fmt.Errorf("DBWriteNewSyncEvent: no transaction provided")
	}

	insertQuery := h.AdaptQuery("INSERT INTO git_sync_status (created, transformerid, envName, appName, status)  VALUES (?, ?, ?, ?, ?);")
	now, err := h.DBReadTransactionTimestamp(ctx, tx)
	if err != nil {
		return fmt.Errorf("DBWriteNewSyncEvent unable to get transaction timestamp: %w", err)
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
		return fmt.Errorf("could not write sync event into DB. Error: %w\n", err)
	}
	return nil
}

func (h *DBHandler) DBWriteNewSyncEventBulk(ctx context.Context, tx *sql.Tx, id TransformerID, envApps []EnvApp, status SyncStatus) error {
	span, ctx := tracer.StartSpanFromContext(ctx, "DBWriteNewSyncEventBulk")
	defer span.Finish()
	if h == nil {
		return nil
	}
	if tx == nil {
		return fmt.Errorf("DBWriteNewSyncEventBulk: no transaction provided")
	}
	now, err := h.DBReadTransactionTimestamp(ctx, tx)
	if err != nil {
		return fmt.Errorf("DBWriteNewSyncEventBulk unable to get transaction timestamp: %w", err)
	}

	err = h.executeBulkInsert(ctx, tx, envApps, *now, id, status, BULK_INSERT_BATCH_SIZE)

	if err != nil {
		return fmt.Errorf("could not write sync event into DB. Error: %w\n", err)
	}
	return nil
}

type EnvApp struct {
	AppName string
	EnvName string
}

func (h *DBHandler) DBReadUnsyncedAppsForTransfomerID(ctx context.Context, tx *sql.Tx, id TransformerID) ([]EnvApp, error) {
	span, ctx := tracer.StartSpanFromContext(ctx, "DBReadUnsyncedAppsForTransfomerID")
	defer span.Finish()
	if h == nil {
		return nil, nil
	}
	if tx == nil {
		return nil, fmt.Errorf("DBReadUnsyncedAppsForTransfomerID: no transaction provided")
	}
	selectQuery := h.AdaptQuery(fmt.Sprintf("SELECT appName, envName FROM git_sync_status WHERE transformerid = ? AND status = ? ORDER BY eslVersion;"))
	rows, err := tx.QueryContext(
		ctx,
		selectQuery,
		id,
		UNSYNCED,
	)
	if err != nil {
		return nil, fmt.Errorf("could not get current eslVersion. Error: %w\n", err)
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
			return nil, fmt.Errorf("Error table for next eslVersion. Error: %w\n", err)
		}
		allCombinations = append(allCombinations, EnvApp{
			AppName: currApp,
			EnvName: currEnv,
		})
	}
	err = closeRows(rows)
	if err != nil {
		return nil, err
	}
	return allCombinations, nil
}

func (h *DBHandler) DBBulkUpdateUnsyncedApps(ctx context.Context, tx *sql.Tx, id TransformerID, status SyncStatus) error {
	span, ctx := tracer.StartSpanFromContext(ctx, "DBBulkUpdateUnsyncedApps")
	defer span.Finish()
	if h == nil {
		return nil
	}
	if tx == nil {
		return fmt.Errorf("DBBulkUpdateUnsyncedApps: no transaction provided")
	}

	allCombs, err := h.DBReadUnsyncedAppsForTransfomerID(ctx, tx, id)
	if err != nil {
		return fmt.Errorf("DBBulkUpdateUnsyncedApps unable to read unsynced apps: %w", err)
	}
	if len(allCombs) == 0 {
		logger.FromContext(ctx).Sugar().Warnf("Could not update all unsynced apps. Did not find any unsynced apps.")
		return nil
	}
	now, err := h.DBReadTransactionTimestamp(ctx, tx)
	if err != nil {
		return fmt.Errorf("DBBulkUpdateUnsyncedApps unable to get transaction timestamp: %w", err)
	}

	return h.executeBulkInsert(ctx, tx, allCombs, *now, id, status, BULK_INSERT_BATCH_SIZE)
}

func (h *DBHandler) DBRetrieveSyncStatus(ctx context.Context, tx *sql.Tx, appName, envName string) (*GitSyncData, error) {
	span, ctx := tracer.StartSpanFromContext(ctx, "DBRetrieveSyncStatus")
	defer span.Finish()
	if h == nil {
		return nil, nil
	}
	if tx == nil {
		return nil, fmt.Errorf("DBRetrieveSyncStatus: no transaction provided")
	}

	selectQuerry := h.AdaptQuery("SELECT transformerid, envName, appName, status FROM git_sync_status WHERE appName = ? AND envName= ? ORDER BY eslVersion DESC LIMIT 1;")
	rows, err := tx.QueryContext(
		ctx,
		selectQuerry,
		appName,
		envName)
	if err != nil {
		return nil, fmt.Errorf("could not get git sync status for %q %q. Error: %w\n", envName, appName, err)
	}

	defer func(rows *sql.Rows) {
		err := rows.Close()
		if err != nil {
			logger.FromContext(ctx).Sugar().Warnf("row closing error: %v", err)
		}
	}(rows)

	var syncData GitSyncData
	if rows.Next() {
		err := rows.Scan(&syncData.TransformerID, &syncData.EnvName, &syncData.AppName, &syncData.SyncStatus)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return nil, nil
			}
			return nil, fmt.Errorf("Error table scanning git_sync_status. Error: %w\n", err)
		}
	}
	err = closeRows(rows)
	if err != nil {
		return nil, err
	}
	return &syncData, nil
}

// These queries can get long. Because of this, we insert these values in batches
func (h *DBHandler) executeBulkInsert(ctx context.Context, tx *sql.Tx, allEnvApps []EnvApp, now time.Time, id TransformerID, status SyncStatus, batchSize int) error {
	queryPrefix := "INSERT INTO git_sync_status (created, transformerid, envName, appName, status) VALUES"
	currentQuery := queryPrefix
	if batchSize < 1 {
		return fmt.Errorf("batch size needs to be a positive number")
	}
	for idx, currComb := range allEnvApps {
		format := " ('%s', %d, '%s', '%s', %d)"
		if idx%batchSize == 0 || idx == len(allEnvApps)-1 { // Is end of batch || tail
			format += ";"
			currentQuery += fmt.Sprintf(format, now.Format(time.RFC3339), id, currComb.EnvName, currComb.AppName, status)
			_, err := tx.ExecContext(ctx, currentQuery)
			if err != nil {
				return err
			}
			currentQuery = queryPrefix
		} else {
			format += ","
			currentQuery += fmt.Sprintf(format, now.Format(time.RFC3339), id, currComb.EnvName, currComb.AppName, status)
		}
	}
	return nil
}
