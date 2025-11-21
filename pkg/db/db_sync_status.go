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
	"github.com/freiheit-com/kuberpult/pkg/types"
	"gopkg.in/DataDog/dd-trace-go.v1/ddtrace/tracer"

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
	AppName       types.AppName
	EnvName       types.EnvName
	TransformerID EslVersion
	SyncStatus
}

func (h *DBHandler) DBWriteNewSyncEvent(ctx context.Context, tx *sql.Tx, syncData *GitSyncData) (err error) {
	span, ctx := tracer.StartSpanFromContext(ctx, "DBWriteNewSyncEvent")
	defer func() {
		span.Finish(tracer.WithError(err))
	}()
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
		return fmt.Errorf("could not write sync event into DB. Error: %w", err)
	}
	return nil
}

func (h *DBHandler) DBWriteNewSyncEventBulk(ctx context.Context, tx *sql.Tx, id TransformerID, envApps []EnvApp, status SyncStatus) (err error) {
	span, ctx := tracer.StartSpanFromContext(ctx, "DBWriteNewSyncEventBulk")
	defer func() {
		span.Finish(tracer.WithError(err))
	}()
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
		return fmt.Errorf("could not write sync event into DB. Error: %w", err)
	}
	return nil
}

type EnvApp struct {
	AppName types.AppName
	EnvName types.EnvName
}

func (h *DBHandler) DBReadUnsyncedAppsForTransfomerID(ctx context.Context, tx *sql.Tx, id TransformerID) (_ []EnvApp, err error) {
	span, ctx := tracer.StartSpanFromContext(ctx, "DBReadUnsyncedAppsForTransfomerID")
	defer func() {
		span.Finish(tracer.WithError(err))
	}()
	if h == nil {
		return nil, nil
	}
	if tx == nil {
		return nil, fmt.Errorf("DBReadUnsyncedAppsForTransfomerID: no transaction provided")
	}

	selectQuery := h.AdaptQuery("SELECT appName, envName FROM git_sync_status WHERE transformerid = ? AND status = ? ORDER BY created DESC;")
	rows, err := tx.QueryContext(
		ctx,
		selectQuery,
		id,
		UNSYNCED,
	)
	if err != nil {
		return nil, fmt.Errorf("could not get current eslVersion. Error: %w", err)
	}
	defer func(rows *sql.Rows) {
		err := rows.Close()
		if err != nil {
			logger.FromContext(ctx).Sugar().Warnf("row closing error: %v", err)
		}
	}(rows)
	allCombinations := make([]EnvApp, 0)
	var currApp types.AppName
	var currEnv types.EnvName
	for rows.Next() {
		err := rows.Scan(&currApp, &currEnv)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return nil, nil
			}
			return nil, fmt.Errorf("error table for next eslVersion. Error: %w", err)
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

func (h *DBHandler) DBReadAllAppsForTransfomerID(ctx context.Context, tx *sql.Tx, id TransformerID) (_ []EnvApp, err error) {
	span, ctx := tracer.StartSpanFromContext(ctx, "DBReadAllAppsForTransfomerID")
	defer func() {
		span.Finish(tracer.WithError(err))
	}()
	if h == nil {
		return nil, nil
	}
	if tx == nil {
		return nil, fmt.Errorf("DBReadAllAppsForTransfomerID: no transaction provided")
	}

	selectQuery := h.AdaptQuery("SELECT appName, envName FROM git_sync_status WHERE transformerid = ? ORDER BY created DESC;")
	rows, err := tx.QueryContext(
		ctx,
		selectQuery,
		id,
	)
	if err != nil {
		return nil, fmt.Errorf("could not get current eslVersion. Error: %w", err)
	}
	defer func(rows *sql.Rows) {
		err := rows.Close()
		if err != nil {
			logger.FromContext(ctx).Sugar().Warnf("row closing error: %v", err)
		}
	}(rows)
	allCombinations := make([]EnvApp, 0)
	var currApp types.AppName
	var currEnv types.EnvName
	for rows.Next() {
		err := rows.Scan(&currApp, &currEnv)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return nil, nil
			}
			return nil, fmt.Errorf("error table for next eslVersion. Error: %w", err)
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

func (h *DBHandler) DBBulkUpdateUnsyncedApps(ctx context.Context, tx *sql.Tx, id TransformerID, status SyncStatus) (err error) {
	span, ctx := tracer.StartSpanFromContext(ctx, "DBBulkUpdateUnsyncedApps")
	defer func() {
		span.Finish(tracer.WithError(err))
	}()
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
		logger.FromContext(ctx).Sugar().Info("Could not update all unsynced apps. Did not find any unsynced apps.")
		return nil
	}
	now, err := h.DBReadTransactionTimestamp(ctx, tx)
	if err != nil {
		return fmt.Errorf("DBBulkUpdateUnsyncedApps unable to get transaction timestamp: %w", err)
	}

	return h.executeBulkInsert(ctx, tx, allCombs, *now, id, status, BULK_INSERT_BATCH_SIZE)
}

func (h *DBHandler) DBBulkUpdateAllApps(ctx context.Context, tx *sql.Tx, newId, oldId TransformerID, status SyncStatus) (err error) {
	span, ctx := tracer.StartSpanFromContext(ctx, "DBBulkUpdateAllApps")
	defer func() {
		span.Finish(tracer.WithError(err))
	}()
	if h == nil {
		return nil
	}
	if tx == nil {
		return fmt.Errorf("DBBulkUpdateAllApps: no transaction provided")
	}

	allCombs, err := h.DBReadAllAppsForTransfomerID(ctx, tx, oldId)
	if err != nil {
		return fmt.Errorf("DBBulkUpdateAllApps unable to read apps: %w", err)
	}
	if len(allCombs) == 0 {
		logger.FromContext(ctx).Sugar().Info("Could not update all apps. Did not find any unsynced apps.")
		return nil
	}
	now, err := h.DBReadTransactionTimestamp(ctx, tx)
	if err != nil {
		return fmt.Errorf("DBBulkUpdateAllApps unable to get transaction timestamp: %w", err)
	}

	return h.executeBulkInsert(ctx, tx, allCombs, *now, newId, status, BULK_INSERT_BATCH_SIZE)
}

func (h *DBHandler) DBBulkUpdateAllDeployments(ctx context.Context, tx *sql.Tx, newId, oldId TransformerID) (err error) {
	span, ctx := tracer.StartSpanFromContext(ctx, "DBBulkUpdateAllDeployments")
	defer func() {
		span.Finish(tracer.WithError(err))
	}()
	if h == nil {
		return nil
	}
	if tx == nil {
		return fmt.Errorf("DBBulkUpdateAllDeployments: no transaction provided")
	}

	query := h.AdaptQuery("UPDATE deployments SET transformereslversion = ? WHERE transformereslversion= ?;")
	_, err = tx.ExecContext(ctx, query, newId, oldId)
	if err != nil {
		return fmt.Errorf("could not update deployments from %q to %q. Error: %w", oldId, newId, err)
	}
	return nil
}

func (h *DBHandler) DBRetrieveAppsByStatus(ctx context.Context, tx *sql.Tx, status SyncStatus) (_ []GitSyncData, err error) {
	span, ctx := tracer.StartSpanFromContext(ctx, "DBRetrieveAppsByStatus")
	defer func() {
		span.Finish(tracer.WithError(err))
	}()
	span.SetTag("status", status)
	selectQuery := h.AdaptQuery("SELECT transformerid, envName, appName, status FROM git_sync_status WHERE status = ? ORDER BY created DESC;")
	rows, err := tx.QueryContext(
		ctx,
		selectQuery,
		status)
	result, err := processGitSyncStatusRows(ctx, rows, err)

	if err != nil {
		return nil, err
	}
	return result, nil
}

func processGitSyncStatusRows(ctx context.Context, rows *sql.Rows, err error) ([]GitSyncData, error) {
	if err != nil {
		return nil, fmt.Errorf("could not get git sync status for apps. Error: %w", err)
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
			return nil, fmt.Errorf("error table scanning git_sync_status. Error: %w", err)
		}
		syncData = append(syncData, curr)
	}
	err = closeRows(rows)
	if err != nil {
		return nil, err
	}
	return syncData, nil
}

func (h *DBHandler) DBRetrieveSyncStatus(ctx context.Context, tx *sql.Tx, appName types.AppName, envName types.EnvName) (_ *GitSyncData, err error) {
	span, ctx := tracer.StartSpanFromContext(ctx, "DBRetrieveSyncStatus")
	defer func() {
		span.Finish(tracer.WithError(err))
	}()
	span.SetTag("app", appName)
	span.SetTag("env", envName)

	selectQuerry := h.AdaptQuery("SELECT transformerid, envName, appName, status FROM git_sync_status WHERE appName = ? AND envName= ? ORDER BY created DESC LIMIT 1;")
	rows, err := tx.QueryContext(
		ctx,
		selectQuerry,
		appName,
		envName)

	result, err := processGitSyncStatusRows(ctx, rows, err)

	if err != nil {
		return nil, err
	}
	if len(result) == 0 {
		return nil, nil
	}
	return &result[0], nil
}

func (h *DBHandler) DBDeleteSyncStatusOnAppAndEnv(ctx context.Context, tx *sql.Tx, appName types.AppName, envName types.EnvName) (err error) {
	span, ctx := tracer.StartSpanFromContext(ctx, "DBDeleteSyncStatusOnAppAndEnv")
	defer func() {
		span.Finish(tracer.WithError(err))
	}()
	span.SetTag("app", appName)
	span.SetTag("env", envName)

	selectQuerry := h.AdaptQuery(`
		DELETE FROM git_sync_status
		WHERE appName = ? AND envName= ?
		;`)
	_, err = tx.ExecContext(
		ctx,
		selectQuerry,
		appName,
		envName)

	if err != nil {
		return fmt.Errorf("delete sync status for app='%s' and env='%s': %w", appName, envName, err)
	}
	return nil
}

// These queries can get long. Because of this, we insert these values in batches
func (h *DBHandler) executeBulkInsert(ctx context.Context, tx *sql.Tx, allEnvApps []EnvApp, now time.Time, id TransformerID, status SyncStatus, batchSize int) (err error) {
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

func (h *DBHandler) DBCountAppsWithStatus(ctx context.Context, tx *sql.Tx, status SyncStatus) (_ int, err error) {
	span, ctx := tracer.StartSpanFromContext(ctx, "DBCountAppsWithStatus")
	defer func() {
		span.Finish(tracer.WithError(err))
	}()
	if h == nil {
		return -1, nil
	}
	if tx == nil {
		return -1, fmt.Errorf("DBCountAppsWithStatus: no transaction provided")
	}

	selectQuerry := h.AdaptQuery("SELECT count(*) FROM git_sync_status WHERE status = (?);")
	rows, err := tx.QueryContext(
		ctx,
		selectQuerry,
		status,
	)

	if err != nil {
		return -1, fmt.Errorf("could not get count of git sync status. Error: %w", err)
	}

	defer func(rows *sql.Rows) {
		err := rows.Close()
		if err != nil {
			logger.FromContext(ctx).Sugar().Warnf("row closing error: %v", err)
		}
	}(rows)

	var count int
	if rows.Next() {
		err := rows.Scan(&count)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return 0, nil
			}
			return -1, fmt.Errorf("error scanning git sync status. Could not retrive number of apps with status %q. Error: %w", status, err)
		}
	}
	err = closeRows(rows)
	if err != nil {
		return -1, err
	}
	return count, nil
}
