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
	"slices"
	"strings"
	"time"

	"github.com/freiheit-com/kuberpult/pkg/types"

	config "github.com/freiheit-com/kuberpult/pkg/config"
	"github.com/freiheit-com/kuberpult/pkg/logger"
	"gopkg.in/DataDog/dd-trace-go.v1/ddtrace/tracer"
)

type DBEnvironment struct {
	Created      time.Time
	Name         types.EnvName
	Config       config.EnvironmentConfig
	Applications []string
}

type DBEnvironmentRow struct {
	Created      time.Time
	Name         types.EnvName
	Config       string
	Applications string
}

// SELECTS
func (h *DBHandler) DBHasAnyEnvironment(ctx context.Context, tx *sql.Tx) (bool, error) {
	selectQuery := h.AdaptQuery(`
		SELECT created, name, json, applications
		FROM environments
		LIMIT 1;
	`)

	rows, err := tx.QueryContext(
		ctx,
		selectQuery,
	)
	if err != nil {
		return false, fmt.Errorf("could not query the environments table for any environment, error: %w", err)
	}
	defer func(rows *sql.Rows) {
		err := rows.Close()
		if err != nil {
			logger.FromContext(ctx).Sugar().Warnf("environment locks: row could not be closed: %v", err)
		}
	}(rows)
	return rows.Next(), nil
}

func (h *DBHandler) DBSelectEnvironment(ctx context.Context, tx *sql.Tx, environmentName types.EnvName) (_ *DBEnvironment, err error) {
	selectQuery := h.AdaptQuery(`
		SELECT created, name, json, applications
		FROM environments
		WHERE name=?
		LIMIT 1;
	`)

	rows, err := tx.QueryContext(
		ctx,
		selectQuery,
		environmentName,
	)

	if err != nil {
		return nil, fmt.Errorf("could not query the environments table for environment %s, error: %w", environmentName, err)
	}
	return h.processEnvironmentRow(ctx, rows)
}

func (h *DBHandler) DBSelectEnvironmentsBatch(ctx context.Context, tx *sql.Tx, environmentNames []types.EnvName) (_ *[]DBEnvironment, err error) {
	span, ctx := tracer.StartSpanFromContext(ctx, "DBSelectEnvironmentsBatch")
	defer span.Finish(tracer.WithError(err))
	if len(environmentNames) > WhereInBatchMax {
		return nil, fmt.Errorf("error: SelectEnvironments is not batching queries for now, make sure to not request more than %d environments", WhereInBatchMax)
	}
	if len(environmentNames) == 0 {
		return &[]DBEnvironment{}, nil
	}
	selectQuery := h.AdaptQuery(`
		SELECT created, name, json, applications
		FROM environments
		WHERE name IN (?` + strings.Repeat(",?", len(environmentNames)-1) + `)
		ORDER BY name
		LIMIT ?
	`)
	span.SetTag("query", selectQuery)
	args := []any{}
	for _, env := range environmentNames {
		args = append(args, env)
	}
	args = append(args, len(environmentNames))
	rows, err := tx.QueryContext(
		ctx,
		selectQuery,
		args...,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to query for environments %v, error: %w (query: %s, args: %v)", environmentNames, err, selectQuery, args)
	}
	return processEnvironmentRows(ctx, rows)
}

func processEnvironmentRows(ctx context.Context, rows *sql.Rows) (*[]DBEnvironment, error) {
	defer func(rows *sql.Rows) {
		err := rows.Close()
		if err != nil {
			logger.FromContext(ctx).Sugar().Warnf("error while closing row of environments, error: %w", err)
		}
	}(rows)

	envs := []DBEnvironment{}
	for rows.Next() {
		//exhaustruct:ignore
		row := DBEnvironmentRow{}
		err := rows.Scan(&row.Created, &row.Name, &row.Config, &row.Applications)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return nil, nil
			}
			return nil, fmt.Errorf("error scanning the environments table, error: %w", err)
		}
		env, err := EnvironmentFromRow(ctx, &row)
		if err != nil {
			return nil, err
		}
		envs = append(envs, *env)
	}
	return &envs, nil
}

func (h *DBHandler) DBSelectAllLatestEnvironmentsAtTimestamp(ctx context.Context, tx *sql.Tx, ts time.Time) (*[]DBEnvironment, error) {
	selectQuery := h.AdaptQuery(`
	SELECT
	    environments_history.created,
		environments_history.name,
		environments_history.json,
		environments_history.applications
	FROM (
	SELECT
		MAX(version) AS latest,
		name
	FROM
		environments_history
	WHERE created <= (?)
	GROUP BY
		name
	) AS latest
	JOIN
		environments_history AS environments_history
	ON
		latest.latest=environments_history.version
		AND latest.name=environments_history.name;
`)

	args := []any{}
	args = append(args, ts)

	rows, err := tx.QueryContext(
		ctx,
		selectQuery,
		args...,
	)

	if err != nil {
		return nil, fmt.Errorf("failed to query for environments, error: %w (query: %s, args: %v)", err, selectQuery, args)
	}
	result, err := processEnvironmentRows(ctx, rows)
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (h *DBHandler) DBSelectAllEnvironments(ctx context.Context, transaction *sql.Tx) (_ []types.EnvName, err error) {
	span, ctx := tracer.StartSpanFromContext(ctx, "DBSelectAllEnvironments")
	defer span.Finish(tracer.WithError(err))

	if h == nil {
		return nil, nil
	}
	if transaction == nil {
		return nil, fmt.Errorf("no transaction provided when selecting all environments from environments table")
	}

	selectQuery := h.AdaptQuery(`
		SELECT name
		FROM environments
		ORDER BY name;
	`)

	rows, err := transaction.QueryContext(ctx, selectQuery)
	if err != nil {
		return nil, fmt.Errorf("error while executing query to get all environments, error: %w", err)
	}

	defer func(rows *sql.Rows) {
		err := rows.Close()
		if err != nil {
			logger.FromContext(ctx).Sugar().Warnf("error while closing row on environments table, error: %w", err)
		}
	}(rows)

	result := []types.EnvName{}
	for rows.Next() {
		//exhaustruct:ignore
		var row types.EnvName
		err := rows.Scan(&row)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return nil, nil
			}
			return nil, fmt.Errorf("error while scanning environments row, error: %w", err)
		}
		result = append(result, row)
	}
	err = closeRows(rows)
	if err != nil {
		return nil, fmt.Errorf("error while closing rows, error: %w", err)
	}
	return result, nil
}

func (h *DBHandler) DBSelectEnvironmentApplications(ctx context.Context, transaction *sql.Tx, envName types.EnvName) (_ []string, err error) {
	span, ctx := tracer.StartSpanFromContext(ctx, "DBSelectEnvironmentApplications")
	defer span.Finish(tracer.WithError(err))

	if h == nil {
		return nil, nil
	}
	if transaction == nil {
		return nil, fmt.Errorf("no transaction provided when selecting environment applications")
	}
	acceptableEnvFormat := `["` + envName + `"]`

	selectQuery := h.AdaptQuery(`
		select DISTINCT appname FROM releases r WHERE
			r.environments @> ?;
	`)

	rows, err := transaction.QueryContext(ctx, selectQuery, acceptableEnvFormat)
	if err != nil {
		return nil, fmt.Errorf("error while executing query to get all environments, error: %w", err)
	}

	defer func(rows *sql.Rows) {
		err := rows.Close()
		if err != nil {
			logger.FromContext(ctx).Sugar().Warnf("error while closing row on releases table, error: %w", err)
		}
	}(rows)

	result := []string{}
	for rows.Next() {
		//exhaustruct:ignore
		var row string
		err := rows.Scan(&row)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return nil, nil
			}
			return nil, fmt.Errorf("error while scanning releases row, error: %w", err)
		}
		result = append(result, row)
	}
	err = closeRows(rows)
	if err != nil {
		return nil, fmt.Errorf("error while closing rows, error: %w", err)
	}
	return result, nil
}

type AppWithTeam struct {
	AppName  types.AppName
	TeamName string
}

func (h *DBHandler) DBSelectEnvironmentApplicationsAtTimestamp(ctx context.Context, tx *sql.Tx, envName types.EnvName, ts time.Time) (_ []string, _ []AppWithTeam, err error) {
	span, ctx := tracer.StartSpanFromContext(ctx, "DBSelectEnvironmentApplicationsAtTimestamp")
	defer span.Finish(tracer.WithError(err))

	selectQuery := h.AdaptQuery(`
	WITH latest_apps AS (
			SELECT
			MAX(version) AS latest,
			appname,
		metadata
		FROM
			apps_history
		WHERE created <= ?
		GROUP BY
			appname, metadata
	),
	
	latest_releases AS (
		SELECT
			MAX(version) AS latest,
			appname,
			releaseversion
		FROM
			releases_history
		WHERE created <= ?
		GROUP BY
			appname, releaseversion
	)

	SELECT latest_apps.appname, latest_apps.metadata
	FROM latest_apps
	JOIN
		apps_history AS apps_history
	ON
		latest_apps.latest=apps_history.version
	WHERE latest_apps.appName in (
		SELECT DISTINCT
			releases_history.appname
		FROM latest_releases
		JOIN
			releases_history AS releases_history
		ON
			latest_releases.latest=releases_history.version
			AND latest_releases.appname=releases_history.appname
			AND latest_releases.releaseversion=releases_history.releaseversion
		WHERE releases_history.environments @> ?
			AND releases_history.deleted=false
	); 
`)
	span.SetTag("query", selectQuery)

	rows, err := tx.QueryContext(
		ctx,
		selectQuery,
		ts,
		ts,
		`["`+envName+`"]`,
	)
	if err != nil {
		return nil, nil, fmt.Errorf("could not query the releases_history table %s, error: %w", envName, err)
	}
	defer func(rows *sql.Rows) {
		err := rows.Close()
		if err != nil {
			logger.FromContext(ctx).Sugar().Warnf("error while closing row on releases_history table, error: %w", err)
		}
	}(rows)

	var appNames = []string{}
	var appNamesWithTeam = []AppWithTeam{}
	for rows.Next() {
		//exhaustruct:ignore
		var metaData = &DBAppMetaData{}
		var row string
		var metadataRaw string
		err := rows.Scan(&row, &metadataRaw)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return nil, nil, nil
			}
			return nil, nil, fmt.Errorf("error while scanning releases_history row, error: %w", err)
		}
		err = json.Unmarshal(([]byte)(metadataRaw), &metaData)
		if err != nil {
			return nil, nil, fmt.Errorf("error during json unmarshal of metadata for releases_history. Error: %w. Data: %s", err, metadataRaw)
		}
		appNames = append(appNames, row)
		appNamesWithTeam = append(appNamesWithTeam, AppWithTeam{
			AppName:  types.AppName(row),
			TeamName: metaData.Team,
		})
	}
	err = closeRows(rows)
	if err != nil {
		return nil, nil, fmt.Errorf("error while closing rows, error: %w", err)
	}
	return appNames, appNamesWithTeam, nil
}

// INSERT, UPDATE, DELETE

func (h *DBHandler) DBWriteEnvironment(ctx context.Context, tx *sql.Tx, environmentName types.EnvName, environmentConfig config.EnvironmentConfig, applications []string) (err error) {
	span, ctx := tracer.StartSpanFromContext(ctx, "DBWriteEnvironment")
	defer span.Finish(tracer.WithError(err))
	err = h.upsertEnvironmentsRow(ctx, tx, environmentName, environmentConfig, applications)
	if err != nil {
		return err
	}
	err = h.insertEnvironmentHistoryRow(ctx, tx, environmentName, environmentConfig, applications, false)
	if err != nil {
		return err
	}
	return nil
}

// DBAppendAppToEnvironment returns an error if the env does not exist yet
func (h *DBHandler) DBAppendAppToEnvironment(ctx context.Context, tx *sql.Tx, environmentName types.EnvName, newApp string) (err error) {
	span, ctx := tracer.StartSpanFromContext(ctx, "DBAppendAppToEnvironment")
	span.SetTag("environment", environmentName)
	span.SetTag("app", newApp)
	defer span.Finish(tracer.WithError(err))
	dbEnv, err := h.addAppToEnvironment(ctx, tx, environmentName, newApp)
	if err != nil {
		return err
	}
	if dbEnv == nil {
		// we did not add the app to the env, because it was already there
		return nil
	}
	err = h.insertEnvironmentHistoryRow(ctx, tx, environmentName, dbEnv.Config, dbEnv.Applications, false)
	if err != nil {
		return err
	}
	return nil
}

// DBRemoveAppFromEnvironment returns an error if the env does not exist yet
func (h *DBHandler) DBRemoveAppFromEnvironment(ctx context.Context, tx *sql.Tx, environmentName types.EnvName, toDeleteApp string) (err error) {
	span, ctx := tracer.StartSpanFromContext(ctx, "DBRemoveAppFromEnvironment")
	span.SetTag("environment", environmentName)
	span.SetTag("app", toDeleteApp)
	defer span.Finish(tracer.WithError(err))
	dbEnv, err := h.deleteAppFromEnvironment(ctx, tx, environmentName, toDeleteApp)
	if err != nil {
		return err
	}
	if dbEnv == nil {
		return fmt.Errorf("remove from env with environment does not exist: '%s'", environmentName)
	}
	err = h.insertEnvironmentHistoryRow(ctx, tx, environmentName, dbEnv.Config, dbEnv.Applications, false)
	if err != nil {
		return err
	}
	return nil
}

func (h *DBHandler) DBDeleteEnvironment(ctx context.Context, tx *sql.Tx, environmentName types.EnvName) (err error) {
	span, _ := tracer.StartSpanFromContext(ctx, "DBDeleteEnvironment")
	defer span.Finish(tracer.WithError(err))

	if h == nil {
		return nil
	}
	if tx == nil {
		return fmt.Errorf("attempting to write to the environments table without a transaction")
	}
	targetEnv, err := h.DBSelectEnvironment(ctx, tx, environmentName)
	if err != nil {
		return err
	}
	if targetEnv == nil {
		return fmt.Errorf("could not delete environment with name '%s' from DB", environmentName)
	}
	err = h.deleteEnvironmentRow(ctx, tx, environmentName)
	if err != nil {
		return err
	}
	err = h.insertEnvironmentHistoryRow(ctx, tx, environmentName, targetEnv.Config, targetEnv.Applications, true)
	if err != nil {
		return err
	}
	return nil
}

// actual changes in tables

func (h *DBHandler) upsertEnvironmentsRow(ctx context.Context, tx *sql.Tx, environmentName types.EnvName, environmentConfig config.EnvironmentConfig, applications []string) (err error) {
	span, _ := tracer.StartSpanFromContext(ctx, "upsertEnvironmentsRow")
	defer span.Finish(tracer.WithError(err))
	if h == nil {
		return nil
	}
	if tx == nil {
		return fmt.Errorf("attempting to write to the environmets table without a transaction")
	}
	insertQuery := h.AdaptQuery(`
		INSERT INTO environments (created, name, json, applications)
		VALUES (?, ?, ?, ?)
		ON CONFLICT(name)
		DO UPDATE SET created = excluded.created, name = excluded.name, json = excluded.json, applications = excluded.applications;
	`)
	span.SetTag("query", insertQuery)
	span.SetTag("queryEnvironment", environmentName)
	span.SetTag("queryApplications", applications)

	jsonToInsert, err := json.Marshal(environmentConfig)
	if err != nil {
		return fmt.Errorf("error while marshalling the environment config %v, error: %w", environmentConfig, err)
	}

	slices.Sort(applications) // we don't really *need* the sorting, it's just for convenience
	applicationsJson, err := json.Marshal(applications)
	if err != nil {
		return fmt.Errorf("could not marshal the application names list %v, error: %w", applicationsJson, err)
	}

	now, err := h.DBReadTransactionTimestamp(ctx, tx)
	if err != nil {
		return fmt.Errorf("DBWriteEnvironment unable to get transaction timestamp: %w", err)
	}
	_, err = tx.Exec(
		insertQuery,
		*now,
		environmentName,
		jsonToInsert,
		string(applicationsJson),
	)
	if err != nil {
		return fmt.Errorf("could not write environment %s with config %v to environments table, error: %w", environmentName, environmentConfig, err)
	}
	return nil
}

// addAppToEnvironment returns the env if the app was added to env, and nil if the app was already there.
// If the env does not exist an error is returned.
func (h *DBHandler) addAppToEnvironment(ctx context.Context, tx *sql.Tx, environmentName types.EnvName, newApp string) (_ *DBEnvironment, err error) {
	span, ctx := tracer.StartSpanFromContext(ctx, "addAppToEnvironment")
	defer span.Finish(tracer.WithError(err))
	if h == nil {
		return nil, fmt.Errorf("addAppToEnvironment: no dbHandler")
	}
	if tx == nil {
		return nil, fmt.Errorf("attempting to write to the environmets table without a transaction")
	}
	// first we check if the env exists.
	// (We could do this in the statement below, but it's easy to confuse "env does not exist" with "app already exists in env",
	//  so we split it up into 2 queries)
	env, err := h.DBSelectEnvironment(ctx, tx, environmentName)
	if err != nil {
		return nil, err
	}
	if env == nil {
		return nil, fmt.Errorf("could not add app to env, env does not exist: %s", environmentName)
	}
	updateQuery := h.AdaptQuery(`
		UPDATE environments
		SET 
			applications = COALESCE(applications::jsonb, '[]'::jsonb) || json_build_array(?::text)::jsonb
		WHERE name = (?)
			AND NOT (applications::jsonb @> json_build_array(to_json(?::text))::jsonb)
		RETURNING created, name, json, applications;
	`)
	span.SetTag("query", updateQuery)
	span.SetTag("queryEnvironment", environmentName)
	span.SetTag("queryNewApp", newApp)

	row, err := tx.QueryContext(
		ctx,
		updateQuery,
		newApp,
		environmentName,
		newApp,
	)
	if err != nil {
		return nil, fmt.Errorf("addAppToEnvironment: could not add app %s to environment %s to environments table, error: %w", newApp, environmentName, err)
	}
	dbEnv, err := h.processEnvironmentRow(ctx, row)
	if err != nil {
		return nil, fmt.Errorf("addAppToEnvironment: could not process row of environment %s and new app %s, error: %w", environmentName, newApp, err)
	}
	if dbEnv == nil {
		return nil, nil
	}
	return dbEnv, nil
}

func (h *DBHandler) deleteAppFromEnvironment(ctx context.Context, tx *sql.Tx, environmentName types.EnvName, deleteThisApp string) (_ *DBEnvironment, err error) {
	span, _ := tracer.StartSpanFromContext(ctx, "deleteAppFromEnvironment")
	defer span.Finish(tracer.WithError(err))
	if h == nil {
		return nil, fmt.Errorf("deleteAppFromEnvironment: no dbHandler")
	}
	if tx == nil {
		return nil, fmt.Errorf("deleteAppFromEnvironment: attempting to write to the environmets table without a transaction")
	}
	updateQuery := h.AdaptQuery(`
		UPDATE environments
		SET 
			applications = COALESCE(applications::jsonb, '[]'::jsonb) - (?)
		WHERE name = (?)
		RETURNING created, name, json, applications;
	`)
	span.SetTag("query", updateQuery)
	span.SetTag("queryEnvironment", environmentName)
	span.SetTag("queryRemovedApp", deleteThisApp)

	row, err := tx.QueryContext(
		ctx,
		updateQuery,
		deleteThisApp,
		environmentName,
	)
	if err != nil {
		return nil, fmt.Errorf("deleteAppFromEnvironment: could not delete app %s to environment %s to environments table, error: %w", deleteThisApp, environmentName, err)
	}
	dbEnv, err := h.processEnvironmentRow(ctx, row)
	if err != nil {
		return nil, fmt.Errorf("deleteAppFromEnvironment: could not process row of environment %s and new app %s, error: %w", environmentName, deleteThisApp, err)
	}
	if dbEnv == nil {
		return nil, nil
	}
	return dbEnv, nil
}

func (h *DBHandler) deleteEnvironmentRow(ctx context.Context, transaction *sql.Tx, environmentName types.EnvName) (err error) {
	span, _ := tracer.StartSpanFromContext(ctx, "deleteEnvironmentRow")
	defer span.Finish(tracer.WithError(err))
	deleteQuery := h.AdaptQuery(`
		DELETE FROM environments WHERE name=? 
	`)
	span.SetTag("query", deleteQuery)
	_, err = transaction.Exec(
		deleteQuery,
		environmentName,
	)
	if err != nil {
		return fmt.Errorf(
			"could not delete environment with name '%s' from DB. Error: %w",
			environmentName,
			err)
	}
	return nil
}

func (h *DBHandler) insertEnvironmentHistoryRow(ctx context.Context, tx *sql.Tx, environmentName types.EnvName, environmentConfig config.EnvironmentConfig, applications []string, deleted bool) (err error) {
	span, _ := tracer.StartSpanFromContext(ctx, "insertEnvironmentHistoryRow")
	defer span.Finish(tracer.WithError(err))
	if h == nil {
		return nil
	}
	if tx == nil {
		return fmt.Errorf("attempting to write to the environmets table without a transaction")
	}
	insertQuery := h.AdaptQuery(`
		INSERT INTO environments_history (created, name, json, applications, deleted)
		VALUES (?, ?, ?, ?, ?);
	`)
	span.SetTag("query", insertQuery)

	jsonToInsert, err := json.Marshal(environmentConfig)
	if err != nil {
		return fmt.Errorf("error while marshalling the environment config %v, error: %w", environmentConfig, err)
	}

	slices.Sort(applications) // we don't really *need* the sorting, it's just for convenience
	applicationsJson, err := json.Marshal(applications)
	if err != nil {
		return fmt.Errorf("could not marshal the application names list %v, error: %w", applicationsJson, err)
	}

	now, err := h.DBReadTransactionTimestamp(ctx, tx)
	if err != nil {
		return fmt.Errorf("DBWriteEnvironment unable to get transaction timestamp: %w", err)
	}
	_, err = tx.Exec(
		insertQuery,
		*now,
		environmentName,
		jsonToInsert,
		string(applicationsJson),
		deleted,
	)
	if err != nil {
		return fmt.Errorf("could not write environment %s with config %v to environments table, error: %w", environmentName, environmentConfig, err)
	}
	return nil
}

// process rows
func (h *DBHandler) processEnvironmentRow(ctx context.Context, rows *sql.Rows) (*DBEnvironment, error) {

	defer func(rows *sql.Rows) {
		err := rows.Close()
		if err != nil {
			logger.FromContext(ctx).Sugar().Warnf("error while closing row of environments, error: %w", err)
		}
	}(rows)

	if rows.Next() {
		//exhaustruct:ignore
		row := DBEnvironmentRow{}
		err := rows.Scan(&row.Created, &row.Name, &row.Config, &row.Applications)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return nil, nil
			}
			return nil, fmt.Errorf("error scanning the environments table, error: %w", err)
		}
		env, err := EnvironmentFromRow(ctx, &row)
		if err != nil {
			return nil, err
		}
		return env, nil
	}
	return nil, nil
}

func EnvironmentFromRow(_ context.Context, row *DBEnvironmentRow) (*DBEnvironment, error) {
	//exhaustruct:ignore
	parsedConfig := config.EnvironmentConfig{}
	err := json.Unmarshal([]byte(row.Config), &parsedConfig)
	if err != nil {
		return nil, fmt.Errorf("unable to unmarshal the JSON in the database, JSON: %s, error: %w", row.Config, err)
	}
	applications := []string{}
	err = json.Unmarshal([]byte(row.Applications), &applications)
	if err != nil {
		return nil, fmt.Errorf("unable to unmarshal the JSON in the database, JSON: %s, error: %w", row.Applications, err)
	}
	return &DBEnvironment{
		Created:      row.Created,
		Name:         row.Name,
		Config:       parsedConfig,
		Applications: applications,
	}, nil
}
