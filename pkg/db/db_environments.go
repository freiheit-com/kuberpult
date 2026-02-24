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

	"gopkg.in/DataDog/dd-trace-go.v1/ddtrace/tracer"

	config "github.com/freiheit-com/kuberpult/pkg/config"
	"github.com/freiheit-com/kuberpult/pkg/logger"
	"github.com/freiheit-com/kuberpult/pkg/types"
)

type DBEnvironment struct {
	Created time.Time
	Name    types.EnvName
	Config  config.EnvironmentConfig
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
	defer func() {
		span.Finish(tracer.WithError(err))
	}()
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
	defer func() {
		span.Finish(tracer.WithError(err))
	}()

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

func (h *DBHandler) DBSelectEnvironmentApplications(ctx context.Context, transaction *sql.Tx, envName types.EnvName) (_ []types.AppName, err error) {
	span, ctx := tracer.StartSpanFromContext(ctx, "DBSelectEnvironmentApplications")
	defer func() {
		span.Finish(tracer.WithError(err))
	}()

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

	result := []types.AppName{}
	for rows.Next() {
		//exhaustruct:ignore
		var row types.AppName
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
	AppName  types.AppName `json:"app"`
	TeamName string        `json:"team"`
}

func (h *DBHandler) DBSelectEnvironmentApplicationsAtTimestamp(ctx context.Context, tx *sql.Tx, envName types.EnvName, ts time.Time) (_ []types.AppName, _ []AppWithTeam, err error) {
	span, ctx := tracer.StartSpanFromContext(ctx, "DBSelectEnvironmentApplicationsAtTimestamp")
	defer func() {
		span.Finish(tracer.WithError(err))
	}()

	appsWithTeam, err := h.DBSelectAppsTeamsHistoryAtTimestamp(ctx, tx, ts)
	if err != nil {
		return nil, nil, fmt.Errorf("could not select apps teams history: %w", err)
	}

	appsWithRelease, err := h.DBSelectAppsWithReleasesAtTimestamp(ctx, tx, envName, ts)
	if err != nil {
		return nil, nil, fmt.Errorf("could not select apps with releases: %w", err)
	}
	appsWithReleaseMap := make(map[types.AppName]struct{})
	for _, app := range appsWithRelease {
		appsWithReleaseMap[app] = struct{}{}
	}

	var appNames = []types.AppName{}
	var appNamesWithTeam = []AppWithTeam{}
	for _, appWithTeam := range appsWithTeam {
		if _, ok := appsWithReleaseMap[appWithTeam.AppName]; ok {
			appNames = append(appNames, appWithTeam.AppName)
			appNamesWithTeam = append(appNamesWithTeam, appWithTeam)
		}
	}

	return appNames, appNamesWithTeam, nil
}

// INSERT, UPDATE, DELETE

func (h *DBHandler) DBWriteEnvironment(ctx context.Context, tx *sql.Tx, environmentName types.EnvName, environmentConfig config.EnvironmentConfig) (err error) {
	span, ctx := tracer.StartSpanFromContext(ctx, "DBWriteEnvironment")
	defer func() {
		span.Finish(tracer.WithError(err))
	}()
	err = h.upsertEnvironmentsRow(ctx, tx, environmentName, environmentConfig)
	if err != nil {
		return err
	}
	err = h.insertEnvironmentHistoryRow(ctx, tx, environmentName, environmentConfig, false)
	if err != nil {
		return err
	}
	return nil
}

func (h *DBHandler) DBDeleteEnvironment(ctx context.Context, tx *sql.Tx, environmentName types.EnvName) (err error) {
	span, _ := tracer.StartSpanFromContext(ctx, "DBDeleteEnvironment")
	defer func() {
		span.Finish(tracer.WithError(err))
	}()

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
	err = h.insertEnvironmentHistoryRow(ctx, tx, environmentName, targetEnv.Config, true)
	if err != nil {
		return err
	}
	return nil
}

// actual changes in tables

func (h *DBHandler) upsertEnvironmentsRow(ctx context.Context, tx *sql.Tx, environmentName types.EnvName, environmentConfig config.EnvironmentConfig) (err error) {
	span, _ := tracer.StartSpanFromContext(ctx, "upsertEnvironmentsRow")
	defer func() {
		span.Finish(tracer.WithError(err))
	}()
	if h == nil {
		return nil
	}
	if tx == nil {
		return fmt.Errorf("attempting to write to the environments table without a transaction")
	}
	insertQuery := h.AdaptQuery(`
		INSERT INTO environments (created, name, json, applications)
		VALUES (?, ?, ?, ?)
		ON CONFLICT(name)
		DO UPDATE SET created = excluded.created, name = excluded.name, json = excluded.json, applications = excluded.applications;
	`)
	span.SetTag("query", insertQuery)
	span.SetTag("queryEnvironment", environmentName)

	jsonToInsert, err := json.Marshal(environmentConfig)
	if err != nil {
		return fmt.Errorf("error while marshalling the environment config %v, error: %w", environmentConfig, err)
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
		"[]",
	)
	if err != nil {
		return fmt.Errorf("could not write environment %s with config %v to environments table, error: %w", environmentName, environmentConfig, err)
	}
	return nil
}

func (h *DBHandler) deleteEnvironmentRow(ctx context.Context, transaction *sql.Tx, environmentName types.EnvName) (err error) {
	span, _ := tracer.StartSpanFromContext(ctx, "deleteEnvironmentRow")
	defer func() {
		span.Finish(tracer.WithError(err))
	}()
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

func (h *DBHandler) insertEnvironmentHistoryRow(ctx context.Context, tx *sql.Tx, environmentName types.EnvName, environmentConfig config.EnvironmentConfig, deleted bool) (err error) {
	span, _ := tracer.StartSpanFromContext(ctx, "insertEnvironmentHistoryRow")
	defer func() {
		span.Finish(tracer.WithError(err))
	}()
	if h == nil {
		return nil
	}
	if tx == nil {
		return fmt.Errorf("attempting to write to the environments table without a transaction")
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

	now, err := h.DBReadTransactionTimestamp(ctx, tx)
	if err != nil {
		return fmt.Errorf("DBWriteEnvironment unable to get transaction timestamp: %w", err)
	}
	_, err = tx.Exec(
		insertQuery,
		*now,
		environmentName,
		jsonToInsert,
		"[]",
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
	applications := []types.AppName{}
	err = json.Unmarshal([]byte(row.Applications), &applications)
	if err != nil {
		return nil, fmt.Errorf("unable to unmarshal the JSON in the database, JSON: %s, error: %w", row.Applications, err)
	}
	return &DBEnvironment{
		Created: row.Created,
		Name:    row.Name,
		Config:  parsedConfig,
	}, nil
}
