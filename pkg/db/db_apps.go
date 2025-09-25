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

	"github.com/freiheit-com/kuberpult/pkg/types"

	"gopkg.in/DataDog/dd-trace-go.v1/ddtrace/tracer"
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
	App        string
}

type DBAppMetaData struct {
	Team string
}

type DBAppWithMetaData struct {
	App         string
	Metadata    DBAppMetaData
	StateChange AppStateChange
}

// SELECTS

func (h *DBHandler) DBSelectApp(ctx context.Context, tx *sql.Tx, appName string) (*DBAppWithMetaData, error) {
	span, ctx := tracer.StartSpanFromContext(ctx, "DBSelectApp")
	defer span.Finish()
	selectQuery := h.AdaptQuery(`
		SELECT appName, stateChange, metadata
		FROM apps
		WHERE appName=? 
		LIMIT 1;
	`)
	span.SetTag("query", selectQuery)

	rows, err := tx.QueryContext(
		ctx,
		selectQuery,
		appName,
	)
	return h.processAppsRow(ctx, rows, err)
}

func (h *DBHandler) DBSelectAllAppsMetadata(ctx context.Context, tx *sql.Tx) (map[types.AppName]*DBAppWithMetaData, error) {
	span, ctx := tracer.StartSpanFromContext(ctx, "DBSelectAllAppsMetadata")
	defer span.Finish()
	selectQuery := h.AdaptQuery(`
		SELECT appname, stateChange, metadata
		FROM apps
		WHERE stateChange <> 'AppStateChangeDelete'
		ORDER BY appname;
	`)
	span.SetTag("query", selectQuery)
	rows, err := tx.QueryContext(ctx, selectQuery)

	return h.processAppsRows(ctx, rows, err)
}

func (h *DBHandler) DBSelectAppAtTimestamp(ctx context.Context, tx *sql.Tx, appName string, ts time.Time) (*DBAppWithMetaData, error) {
	span, ctx := tracer.StartSpanFromContext(ctx, "DBSelectAppAtTimestamp")
	defer span.Finish()
	selectQuery := h.AdaptQuery(`
		SELECT appName, stateChange, metadata
		FROM apps_history
		WHERE appName=? AND created <= ?
		ORDER BY version DESC 
		LIMIT 1;
	`)
	span.SetTag("query", selectQuery)

	rows, err := tx.QueryContext(
		ctx,
		selectQuery,
		appName,
		ts,
	)
	return h.processAppsRow(ctx, rows, err)
}

func (h *DBHandler) DBSelectExistingApp(ctx context.Context, tx *sql.Tx, appName string) (*DBAppWithMetaData, error) {
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

func (h *DBHandler) DBSelectAllApplications(ctx context.Context, transaction *sql.Tx) ([]string, error) {
	if h == nil {
		return nil, nil
	}
	if transaction == nil {
		return nil, fmt.Errorf("DBSelectAllEventsForCommit: no transaction provided")
	}
	span, ctx := tracer.StartSpanFromContext(ctx, "DBSelectAllApplications")
	defer span.Finish()
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
func (h *DBHandler) DBInsertOrUpdateApplication(ctx context.Context, transaction *sql.Tx, appName string, stateChange AppStateChange, metaData DBAppMetaData) error {
	span, ctx := tracer.StartSpanFromContext(ctx, "DBInsertOrUpdateApplication")
	defer span.Finish()
	err := h.upsertAppsRow(ctx, transaction, appName, stateChange, metaData)
	if err != nil {
		return err
	}
	err = h.insertAppsHistoryRow(ctx, transaction, appName, stateChange, metaData)
	if err != nil {
		return err
	}
	return nil
}

// actual changes in tables
func (h *DBHandler) upsertAppsRow(ctx context.Context, transaction *sql.Tx, appName string, stateChange AppStateChange, metaData DBAppMetaData) error {
	span, ctx := tracer.StartSpanFromContext(ctx, "upsertAppsRow")
	defer span.Finish()
	upsertQuery := h.AdaptQuery(`
		INSERT INTO apps (created, appName, stateChange, metadata)
		VALUES (?, ?, ?, ?)
		ON CONFLICT(appname)
		DO UPDATE SET created = excluded.created, appname = excluded.appname, statechange = excluded.statechange, metadata = excluded.metadata;
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
	_, err = transaction.Exec(
		upsertQuery,
		*now,
		appName,
		stateChange,
		jsonToInsert,
	)
	if err != nil {
		return fmt.Errorf("could not upsert app %s into DB. Error: %w", appName, err)
	}
	return nil
}

func (h *DBHandler) insertAppsHistoryRow(ctx context.Context, transaction *sql.Tx, appName string, stateChange AppStateChange, metaData DBAppMetaData) error {
	span, ctx := tracer.StartSpanFromContext(ctx, "insertAppsHistoryRow")
	defer span.Finish()
	insertQuery := h.AdaptQuery(`
		INSERT INTO apps_history (created, appName, stateChange, metadata)
		VALUES (?, ?, ?, ?);
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
	_, err = transaction.Exec(
		insertQuery,
		*now,
		appName,
		stateChange,
		jsonToInsert,
	)
	if err != nil {
		return fmt.Errorf("could not upsert app %s into DB. Error: %w", appName, err)
	}
	return nil
}

// process rows functions

func (h *DBHandler) processAppsRow(ctx context.Context, rows *sql.Rows, err error) (*DBAppWithMetaData, error) {
	if err != nil {
		return nil, fmt.Errorf("could not query apps table from DB. Error: %w", err)
	}
	defer closeRowsAndLog(rows, ctx, "apps")
	//exhaustruct:ignore
	var row = &DBAppWithMetaData{}
	if rows.Next() {
		var metadataStr string
		err := rows.Scan(&row.App, &row.StateChange, &metadataStr)
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
		row.Metadata = metaData
	} else {
		row = nil
	}
	return row, nil
}

func (h *DBHandler) processAppsRows(ctx context.Context, rows *sql.Rows, err error) (map[types.AppName]*DBAppWithMetaData, error) {

	if err != nil {
		return nil, fmt.Errorf("could not query apps table from DB. Error: %w", err)
	}
	defer closeRowsAndLog(rows, ctx, "apps")
	result := make(map[types.AppName]*DBAppWithMetaData)
	for rows.Next() {
		//exhaustruct:ignore
		var row = &DBAppWithMetaData{}
		var metadataStr string
		err := rows.Scan(&row.App, &row.StateChange, &metadataStr)
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
		row.Metadata = metaData
		result[types.AppName(row.App)] = row
	}
	return result, nil
}

func (h *DBHandler) processAllAppsRows(ctx context.Context, rows *sql.Rows, err error) ([]string, error) {
	if err != nil {
		return nil, fmt.Errorf("could not query apps table from DB. Error: %w", err)
	}
	defer closeRowsAndLog(rows, ctx, "apps")
	var result = make([]string, 0)
	for rows.Next() {
		//exhaustruct:ignore
		var appname string
		err := rows.Scan(&appname)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return nil, nil
			}
			return nil, fmt.Errorf("error scanning apps row from DB. Error: %w", err)
		}
		result = append(result, appname)
	}
	return result, nil
}
