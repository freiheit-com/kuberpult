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

type AppStateChange string

const (
	InitialEslVersion EslVersion = 1

	AppStateChangeMigrate AppStateChange = "AppStateChangeMigrate"
	AppStateChangeCreate  AppStateChange = "AppStateChangeCreate"
	AppStateChangeUpdate  AppStateChange = "AppStateChangeUpdate"
	AppStateChangeDelete  AppStateChange = "AppStateChangeDelete"
)

type DBApp struct {
	EslVersion EslVersion
	App        types.AppName
}

type DBAppMetaData struct {
	Team string
}

type DBAppWithMetaData struct {
	App         types.AppName
	Metadata    DBAppMetaData
	StateChange AppStateChange
	ArgoBracket types.ArgoBracketName
}

// SELECTS

func (h *DBHandler) DBSelectApp(ctx context.Context, tx *sql.Tx, appName types.AppName) (*DBAppWithMetaData, error) {
	selectQuery := h.AdaptQuery(`
		SELECT appName, stateChange, metadata, argoBracket
		FROM apps
		WHERE appName=? 
		LIMIT 1;
	`)

	rows, err := tx.QueryContext(
		ctx,
		selectQuery,
		appName,
	)
	return h.processAppsRow(ctx, rows, err)
}

func (h *DBHandler) DBSelectAllAppsMetadata(ctx context.Context, tx *sql.Tx) (_ map[types.AppName]*DBAppWithMetaData, err error) {
	span, ctx := tracer.StartSpanFromContext(ctx, "DBSelectAllAppsMetadata")
	defer func() {
		span.Finish(tracer.WithError(err))
	}()
	selectQuery := h.AdaptQuery(`
		SELECT appname, stateChange, metadata, argoBracket
		FROM apps
		WHERE stateChange <> 'AppStateChangeDelete'
		ORDER BY appname;
	`)
	span.SetTag("query", selectQuery)
	rows, err := tx.QueryContext(ctx, selectQuery)

	return h.processAppsRows(ctx, rows, err)
}

func (h *DBHandler) DBSelectAppAtTimestamp(ctx context.Context, tx *sql.Tx, appName types.AppName, ts time.Time) (*DBAppWithMetaData, error) {
	selectQuery := h.AdaptQuery(`
		SELECT appName, stateChange, metadata, argoBracket
		FROM apps_history
		WHERE appName=? AND created <= ?
		ORDER BY version DESC 
		LIMIT 1;
	`)

	rows, err := tx.QueryContext(
		ctx,
		selectQuery,
		appName,
		ts,
	)
	return h.processAppsRow(ctx, rows, err)
}

func (h *DBHandler) DBSelectExistingApp(ctx context.Context, tx *sql.Tx, appName types.AppName) (*DBAppWithMetaData, error) {
	app, err := h.DBSelectApp(ctx, tx, appName)
	if err != nil {
		return nil, err
	}
	if app == nil {
		return nil, nil
	}
	if app.StateChange == AppStateChangeDelete {
		return nil, nil
	}
	return app, nil
}

func (h *DBHandler) DBSelectAllApplications(ctx context.Context, transaction *sql.Tx) (_ []types.AppName, err error) {
	if h == nil {
		return nil, nil
	}
	if transaction == nil {
		return nil, fmt.Errorf("DBSelectAllEventsForCommit: no transaction provided")
	}
	span, ctx := tracer.StartSpanFromContext(ctx, "DBSelectAllApplications")
	defer func() {
		span.Finish(tracer.WithError(err))
	}()
	query := h.AdaptQuery(`
		SELECT appname
		FROM apps
		WHERE stateChange <> 'AppStateChangeDelete'
		ORDER BY appname;
	`)
	span.SetTag("query", query)
	rows, err := transaction.QueryContext(ctx, query)
	return h.processAllAppsRows(ctx, rows, err)
}

// INSERT, UPDATE, DELETE
func (h *DBHandler) DBInsertOrUpdateApplication(ctx context.Context, transaction *sql.Tx, appName types.AppName, stateChange AppStateChange, metaData DBAppMetaData, argoBracket types.ArgoBracketName) (err error) {
	span, ctx := tracer.StartSpanFromContext(ctx, "DBInsertOrUpdateApplication")
	defer func() {
		span.Finish(tracer.WithError(err))
	}()
	err = h.upsertAppsRow(ctx, transaction, appName, stateChange, metaData, argoBracket)
	if err != nil {
		return fmt.Errorf("upsertAppRow: %w", err)
	}
	err = h.insertAppsHistoryRow(ctx, transaction, appName, stateChange, metaData, argoBracket)
	if err != nil {
		return fmt.Errorf("insertAppsHistoryRow: %w", err)
	}
	return nil
}

// actual changes in tables
func (h *DBHandler) upsertAppsRow(ctx context.Context, transaction *sql.Tx, appName types.AppName, stateChange AppStateChange, metaData DBAppMetaData, argoBracket types.ArgoBracketName) (err error) {
	span, ctx := tracer.StartSpanFromContext(ctx, "upsertAppsRow")
	defer func() {
		span.Finish(tracer.WithError(err))
	}()
	upsertQuery := h.AdaptQuery(`
		INSERT INTO apps (created, appName, stateChange, metadata, argoBracket)
		VALUES (?, ?, ?, ?, ?)
		ON CONFLICT(appname)
		DO UPDATE SET created = excluded.created,
		    appname = excluded.appname,
		    statechange = excluded.statechange,
		    metadata = excluded.metadata,
		    argoBracket = excluded.argoBracket;
	`)
	span.SetTag("query", upsertQuery)

	jsonToInsert, err := json.Marshal(metaData)
	if err != nil {
		return fmt.Errorf("could not marshal json data: %w", err)
	}
	now, err := h.DBReadTransactionTimestamp(ctx, transaction)
	if err != nil {
		return fmt.Errorf("upsertAppsRow unable to get transaction timestamp: %w", err)
	}
	nullableBracket := argoBracketToSql(argoBracket)
	_, err = transaction.Exec(
		upsertQuery,
		*now,
		appName,
		stateChange,
		jsonToInsert,
		nullableBracket,
	)
	if err != nil {
		return fmt.Errorf("could not upsert app %s into DB. Error: %w", appName, err)
	}
	return nil
}

func (h *DBHandler) insertAppsHistoryRow(ctx context.Context, transaction *sql.Tx, appName types.AppName, stateChange AppStateChange, metaData DBAppMetaData, argoBracket types.ArgoBracketName) (err error) {
	span, ctx := tracer.StartSpanFromContext(ctx, "insertAppsHistoryRow")
	defer func() {
		span.Finish(tracer.WithError(err))
	}()
	insertQuery := h.AdaptQuery(`
		INSERT INTO apps_history (created, appName, stateChange, metadata, argoBracket)
		VALUES (?, ?, ?, ?, ?);
	`)
	span.SetTag("query", insertQuery)

	jsonToInsert, err := json.Marshal(metaData)
	if err != nil {
		return fmt.Errorf("could not marshal json data: %w", err)
	}
	now, err := h.DBReadTransactionTimestamp(ctx, transaction)
	if err != nil {
		return fmt.Errorf("upsertAppsRow unable to get transaction timestamp: %w", err)
	}

	nullableBracket := argoBracketToSql(argoBracket)
	_, err = transaction.Exec(
		insertQuery,
		*now,
		appName,
		stateChange,
		jsonToInsert,
		nullableBracket,
	)
	if err != nil {
		return fmt.Errorf("could not upsert app %s into DB. Error: %w", appName, err)
	}
	return nil
}

func argoBracketToSql(argoBracket types.ArgoBracketName) *string {
	if argoBracket == "" {
		return nil
	}
	return types.Ptr(string(argoBracket))
}

func sqlToArgoBracket(sqlData sql.NullString) types.ArgoBracketName {
	if sqlData.Valid {
		return types.ArgoBracketName(sqlData.String)
	}
	return ""
}

// process rows functions

func (h *DBHandler) processAppsRow(ctx context.Context, rows *sql.Rows, err error) (*DBAppWithMetaData, error) {
	if err != nil {
		return nil, fmt.Errorf("could not query apps table from DB. Error: %w", err)
	}
	defer closeRowsAndLog(rows, ctx, "apps")
	var row *DBAppWithMetaData
	if rows.Next() {
		row, err = h.processOneAppInternal(rows)
		if err != nil {
			return nil, fmt.Errorf("processOneAppInternal: %w", err)
		}
	} else {
		row = nil
	}
	err = closeRows(rows)
	if err != nil {
		return nil, err
	}
	return row, nil
}

func (h *DBHandler) processOneAppInternal(rows *sql.Rows) (*DBAppWithMetaData, error) {
	var metadataStr string
	rawBracket := sql.NullString{}
	result := &DBAppWithMetaData{}
	err := rows.Scan(&result.App, &result.StateChange, &metadataStr, &rawBracket)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("error scanning apps row from DB. Error: %w", err)
	}
	var metaData = DBAppMetaData{Team: ""}
	err = json.Unmarshal(([]byte)(metadataStr), &metaData)
	if err != nil {
		return nil, fmt.Errorf("error during json unmarshal of apps. Error: %w. Data: %s", err, metadataStr)
	}
	result.Metadata = metaData
	result.ArgoBracket = sqlToArgoBracket(rawBracket)
	return result, nil
}

func (h *DBHandler) processAppsRows(ctx context.Context, rows *sql.Rows, err error) (map[types.AppName]*DBAppWithMetaData, error) {
	if err != nil {
		return nil, fmt.Errorf("could not query apps table from DB. Error: %w", err)
	}
	defer closeRowsAndLog(rows, ctx, "apps")
	result := make(map[types.AppName]*DBAppWithMetaData)
	for rows.Next() {
		//exhaustruct:ignore
		row, err := h.processOneAppInternal(rows)
		if err != nil {
			return nil, fmt.Errorf("processOneAppInternal: %w", err)
		}
		if row != nil {
			result[row.App] = row
		}
	}
	err = closeRows(rows)
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (h *DBHandler) processAllAppsRows(ctx context.Context, rows *sql.Rows, err error) ([]types.AppName, error) {
	if err != nil {
		return nil, fmt.Errorf("could not query apps table from DB. Error: %w", err)
	}
	defer closeRowsAndLog(rows, ctx, "apps")
	var result = make([]types.AppName, 0)
	for rows.Next() {
		//exhaustruct:ignore
		var appname types.AppName
		err := rows.Scan(&appname)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return nil, nil
			}
			return nil, fmt.Errorf("error scanning apps row from DB. Error: %w", err)
		}
		result = append(result, appname)
	}
	err = closeRows(rows)
	if err != nil {
		return nil, err
	}
	return result, nil
}
