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
}

// SELECTS

func (h *DBHandler) DBSelectApp(ctx context.Context, tx *sql.Tx, appName types.AppName) (*DBAppWithMetaData, error) {
	selectQuery := h.AdaptQuery(`
		SELECT appName, stateChange, metadata
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
		SELECT appname, stateChange, metadata
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
		SELECT appName, stateChange, metadata
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
func (h *DBHandler) DBInsertOrUpdateApplication(ctx context.Context, transaction *sql.Tx, appName types.AppName, stateChange AppStateChange, metaData DBAppMetaData) (err error) {
	span, ctx := tracer.StartSpanFromContext(ctx, "DBInsertOrUpdateApplication")
	defer func() {
		span.Finish(tracer.WithError(err))
	}()
	err = h.upsertAppsRow(ctx, transaction, appName, stateChange, metaData)
	if err != nil {
		return err
	}
	err = h.insertAppsHistoryRow(ctx, transaction, appName, stateChange, metaData)
	if err != nil {
		return err
	}
	return h.DBInsertAppsTeamsHistory(ctx, transaction, appName, metaData.Team, stateChange, nil)
}

func (h *DBHandler) DBInsertAppsTeamsHistory(ctx context.Context, tx *sql.Tx, appName types.AppName, teamName string, stateChange AppStateChange, ts *time.Time) (err error) {
	span, ctx := tracer.StartSpanFromContext(ctx, "DBInsertAppsTeamsHistory")
	defer func() {
		span.Finish(tracer.WithError(err))
	}()
	latestAppsWithTeams, err := h.DBSelectLatestAppsTeamsHistory(ctx, tx)
	if err != nil {
		return err
	}

	var toInsert []AppWithTeam
	var latestApp *AppWithTeam
	for _, appWithTeam := range latestAppsWithTeams {
		if appWithTeam.AppName == appName {
			latestApp = &appWithTeam
			break
		}
	}

	if stateChange == AppStateChangeCreate || stateChange == AppStateChangeMigrate || (stateChange == AppStateChangeUpdate && latestApp == nil) {
		toInsert = append(latestAppsWithTeams, AppWithTeam{
			AppName:  appName,
			TeamName: teamName,
		})
	}

	if latestApp != nil {
		for _, appWithTeam := range latestAppsWithTeams {
			if appWithTeam.AppName != appName {
				toInsert = append(toInsert, appWithTeam)
			} else if stateChange == AppStateChangeUpdate {
				toInsert = append(toInsert, AppWithTeam{
					AppName:  appName,
					TeamName: teamName,
				})
			}
		}
	}

	if ts == nil {
		// get current timestamp
		ts, err = h.DBReadTransactionTimestamp(ctx, tx)
		if err != nil {
			return fmt.Errorf("unable to get transaction timestamp: %w", err)
		}
	}

	err = h.insertAppsTeamsHistoryRow(ctx, tx, toInsert, ts)
	if err != nil {
		return err
	}
	return nil
}

type AppHistoryRow struct {
	Created     time.Time
	AppName     types.AppName
	StateChange AppStateChange
	Metadata    DBAppMetaData
}

func (h *DBHandler) DBMigrateAppsHistoryToAppsTeamsHistory(ctx context.Context, tx *sql.Tx) (err error) {
	span, ctx := tracer.StartSpanFromContext(ctx, "DBMigrateAppsHistoryToAppsTeamsHistory")
	defer func() {
		span.Finish(tracer.WithError(err))
	}()

	var appsHistoryRows []AppHistoryRow
	selectQuery := h.AdaptQuery(`
		SELECT created, appName, stateChange, metadata
		FROM apps_history
		ORDER BY version ASC;
	`)
	rows, err := tx.QueryContext(
		ctx,
		selectQuery,
	)
	if err != nil {
		return err
	}

	for rows.Next() {
		appHistoryRow := AppHistoryRow{}
		var metadataJson string
		err = rows.Scan(&appHistoryRow.Created, &appHistoryRow.AppName, &appHistoryRow.StateChange, &metadataJson)
		if err != nil {
			return err
		}
		err = json.Unmarshal([]byte(metadataJson), &appHistoryRow.Metadata)
		if err != nil {
			return err
		}
		appsHistoryRows = append(appsHistoryRows, appHistoryRow)
	}
	err = closeRows(rows)
	if err != nil {
		return err
	}

	for _, appHistoryRow := range appsHistoryRows {
		err = h.DBInsertAppsTeamsHistory(ctx, tx, appHistoryRow.AppName, appHistoryRow.Metadata.Team, appHistoryRow.StateChange, &appHistoryRow.Created)
		if err != nil {
			return err
		}
	}

	return nil
}

func (h *DBHandler) insertAppsTeamsHistoryRow(ctx context.Context, transaction *sql.Tx, appsWithTeams []AppWithTeam, ts *time.Time) (err error) {
	insertQuery := h.AdaptQuery(`
		INSERT INTO apps_teams_history (created_at, apps_teams)
		VALUES (?, ?);
	`)

	jsonToInsert, err := json.Marshal(appsWithTeams)
	if err != nil {
		return fmt.Errorf("could not marshal json data: %w", err)
	}

	_, err = transaction.ExecContext(
		ctx,
		insertQuery,
		*ts,
		jsonToInsert,
	)
	if err != nil {
		return fmt.Errorf("could not insert new row into apps_teams_history table. Error: %w", err)
	}
	return nil
}

func (h *DBHandler) DBSelectAppsWithReleasesAtTimestamp(ctx context.Context, transaction *sql.Tx, envName types.EnvName, ts time.Time) ([]types.AppName, error) {
	query := h.AdaptQuery(`
	SELECT DISTINCT appname
	FROM (
		SELECT DISTINCT ON (appname, releaseversion, revision)
			appname,
			environments,
			deleted
		FROM releases_history
		WHERE created <= ?
		ORDER BY
			appname,
			releaseversion,
			revision,
			version DESC
	) AS latest_releases
	WHERE
		environments @> ?
		AND deleted = false;
	`)
	rows, err := transaction.QueryContext(ctx, query, ts, `"`+envName+`"`)
	if err != nil {
		return nil, fmt.Errorf("could not query apps with releases at timestamp: %w", err)
	}

	var apps []types.AppName
	for rows.Next() {
		var appName types.AppName
		if err := rows.Scan(&appName); err != nil {
			return nil, fmt.Errorf("could not scan apps with releases at timestamp: %w", err)
		}
		apps = append(apps, appName)
	}

	err = closeRows(rows)
	if err != nil {
		return nil, err
	}
	return apps, nil
}

func (h *DBHandler) DBSelectLatestAppsTeamsHistory(ctx context.Context, transaction *sql.Tx) (_ []AppWithTeam, err error) {
	query := h.AdaptQuery(`
		SELECT apps_teams
		FROM apps_teams_history
		ORDER BY id DESC
		LIMIT 1;
	`)
	rows, err := transaction.QueryContext(ctx, query)
	return h.processAppsTeamsRow(rows, err)
}

func (h *DBHandler) DBSelectAppsTeamsHistoryAtTimestamp(ctx context.Context, transaction *sql.Tx, ts time.Time) (_ []AppWithTeam, err error) {
	query := h.AdaptQuery(`
		SELECT apps_teams
		FROM apps_teams_history
		WHERE created_at <= ?
		ORDER BY id DESC
		LIMIT 1;
	`)
	rows, err := transaction.QueryContext(ctx, query, ts)
	return h.processAppsTeamsRow(rows, err)
}

func (h *DBHandler) processAppsTeamsRow(rows *sql.Rows, err error) ([]AppWithTeam, error) {
	if err != nil {
		return nil, fmt.Errorf("could not query apps_teams_history table. Error: %w", err)
	}

	appsWithTeam := make([]AppWithTeam, 0)
	for rows.Next() {
		var appsTeamsJson string
		if err := rows.Scan(&appsTeamsJson); err != nil {
			return nil, err
		}
		if err := json.Unmarshal([]byte(appsTeamsJson), &appsWithTeam); err != nil {
			return nil, err
		}
	}
	err = closeRows(rows)
	if err != nil {
		return nil, err
	}
	return appsWithTeam, nil
}

// actual changes in tables
func (h *DBHandler) upsertAppsRow(ctx context.Context, transaction *sql.Tx, appName types.AppName, stateChange AppStateChange, metaData DBAppMetaData) (err error) {
	upsertQuery := h.AdaptQuery(`
		INSERT INTO apps (created, appName, stateChange, metadata)
		VALUES (?, ?, ?, ?)
		ON CONFLICT(appname)
		DO UPDATE SET created = excluded.created, appname = excluded.appname, statechange = excluded.statechange, metadata = excluded.metadata;
	`)

	jsonToInsert, err := json.Marshal(metaData)
	if err != nil {
		return fmt.Errorf("could not marshal json data: %w", err)
	}
	now, err := h.DBReadTransactionTimestamp(ctx, transaction)
	if err != nil {
		return fmt.Errorf("upsertAppsRow unable to get transaction timestamp: %w", err)
	}
	_, err = transaction.ExecContext(
		ctx,
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

func (h *DBHandler) insertAppsHistoryRow(ctx context.Context, transaction *sql.Tx, appName types.AppName, stateChange AppStateChange, metaData DBAppMetaData) (err error) {
	insertQuery := h.AdaptQuery(`
		INSERT INTO apps_history (created, appName, stateChange, metadata)
		VALUES (?, ?, ?, ?);
	`)

	jsonToInsert, err := json.Marshal(metaData)
	if err != nil {
		return fmt.Errorf("could not marshal json data: %w", err)
	}
	now, err := h.DBReadTransactionTimestamp(ctx, transaction)
	if err != nil {
		return fmt.Errorf("upsertAppsRow unable to get transaction timestamp: %w", err)
	}
	_, err = transaction.ExecContext(
		ctx,
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
	err = closeRows(rows)
	if err != nil {
		return nil, err
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
		result[row.App] = row
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
