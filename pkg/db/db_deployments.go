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

type DBDeployment struct {
	Created        time.Time
	ReleaseVersion *int64
	App            string
	Env            string
	TransformerID  TransformerID
	Metadata       string // json
}

type DeploymentMetadata struct {
	DeployedByName  string
	DeployedByEmail string
	CiLink          string
}

type Deployment struct {
	Created       time.Time
	App           string
	Env           string
	Version       *int64
	Metadata      DeploymentMetadata
	TransformerID TransformerID
}

// SELECT

func (h *DBHandler) DBSelectLatestDeployment(ctx context.Context, tx *sql.Tx, appSelector string, envSelector string) (*Deployment, error) {
	span, ctx := tracer.StartSpanFromContext(ctx, "DBSelectLatestDeployment")
	defer span.Finish()
	selectQuery := h.AdaptQuery(`
		SELECT created, releaseVersion, appName, envName, metadata, transformereslVersion
		FROM deployments
		WHERE appName=? AND envName=?
		LIMIT 1;
	`)
	span.SetTag("query", selectQuery)
	rows, err := tx.QueryContext(
		ctx,
		selectQuery,
		appSelector,
		envSelector,
	)
	if err != nil {
		return nil, fmt.Errorf("could not select deployment from DB. Error: %w\n", err)
	}
	defer func(rows *sql.Rows) {
		err := rows.Close()
		if err != nil {
			logger.FromContext(ctx).Sugar().Warnf("deployments: row closing error: %v", err)
		}
	}(rows)
	return processDeployment(rows)
}

func (h *DBHandler) DBSelectAllLatestDeploymentsForApplication(ctx context.Context, tx *sql.Tx, appName string) (map[string]Deployment, error) {
	span, ctx := tracer.StartSpanFromContext(ctx, "DBSelectAllLatestDeploymentsForApplication")
	defer span.Finish()
	selectQuery := h.AdaptQuery(`
		SELECT created, appname, releaseVersion, envName, metadata
		FROM deployments
		WHERE deployments.appname = (?) AND deployments.releaseVersion IS NOT NULL;
	`)
	span.SetTag("query", selectQuery)
	rows, err := tx.QueryContext(
		ctx,
		selectQuery,
		appName,
	)
	if err != nil {
		return nil, fmt.Errorf("could not select deployment from DB. Error: %w\n", err)
	}
	defer func(rows *sql.Rows) {
		err := rows.Close()
		if err != nil {
			logger.FromContext(ctx).Sugar().Warnf("deployments: row closing error: %v", err)
		}
	}(rows)
	return processAllLatestDeploymentsForApp(rows)
}

func (h *DBHandler) DBSelectAllLatestDeploymentsOnEnvironment(ctx context.Context, tx *sql.Tx, envName string) (map[string]*int64, error) {
	span, ctx := tracer.StartSpanFromContext(ctx, "DBSelectAllLatestDeploymentsOnEnvironment")
	defer span.Finish()
	selectQuery := h.AdaptQuery(`
		SELECT appname, releaseVersion
		FROM deployments
		WHERE deployments.envName= ?;
	`)

	span.SetTag("query", selectQuery)
	rows, err := tx.QueryContext(
		ctx,
		selectQuery,
		envName,
	)
	if err != nil {
		return nil, fmt.Errorf("could not select deployment from DB. Error: %w\n", err)
	}
	defer func(rows *sql.Rows) {
		err := rows.Close()
		if err != nil {
			logger.FromContext(ctx).Sugar().Warnf("deployments: row closing error: %v", err)
		}
	}(rows)
	return processAllLatestDeployments(rows)
}

func (h *DBHandler) DBSelectSpecificDeployment(ctx context.Context, tx *sql.Tx, appSelector string, envSelector string, releaseVersion uint64) (*Deployment, error) {
	span, ctx := tracer.StartSpanFromContext(ctx, "DBSelectSpecificDeployment")
	defer span.Finish()
	selectQuery := h.AdaptQuery(`
		SELECT created, releaseVersion, appName, envName, metadata, transformereslVersion
		FROM deployments
		WHERE appName=? AND envName=? and releaseVersion=?
		LIMIT 1;
	`)

	span.SetTag("query", selectQuery)
	rows, err := tx.QueryContext(
		ctx,
		selectQuery,
		appSelector,
		envSelector,
		releaseVersion,
	)
	if err != nil {
		return nil, fmt.Errorf("could not select deployment from DB. Error: %w\n", err)
	}
	defer func(rows *sql.Rows) {
		err := rows.Close()
		if err != nil {
			logger.FromContext(ctx).Sugar().Warnf("deployments: row closing error: %v", err)
		}
	}(rows)
	return processDeployment(rows)
}

func (h *DBHandler) DBSelectDeploymentHistory(ctx context.Context, tx *sql.Tx, appSelector string, envSelector string, limit int) ([]Deployment, error) {
	span, ctx := tracer.StartSpanFromContext(ctx, "DBSelectDeploymentHistory")
	defer span.Finish()
	selectQuery := h.AdaptQuery(`
		SELECT created, releaseVersion, appName, envname, metadata, transformereslversion
		FROM deployments_history
		WHERE deployments_history.appname = (?) AND deployments_history.envname = (?)
		ORDER BY version DESC
		LIMIT ?;
	`)

	span.SetTag("query", selectQuery)
	rows, err := tx.QueryContext(
		ctx,
		selectQuery,
		appSelector,
		envSelector,
		limit,
	)
	if err != nil {
		return nil, fmt.Errorf("could not select deployment history from DB. Error: %w\n", err)
	}
	defer func(rows *sql.Rows) {
		err := rows.Close()
		if err != nil {
			logger.FromContext(ctx).Sugar().Warnf("deployments: row closing error: %v", err)
		}
	}(rows)

	result := make([]Deployment, 0)

	for rows.Next() {
		row, err := h.processSingleDeploymentRow(ctx, rows)
		if err != nil {
			return nil, err
		}
		result = append(result, *row)
	}
	err = closeRows(rows)
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (h *DBHandler) DBSelectDeploymentsByTransformerID(ctx context.Context, tx *sql.Tx, transformerID TransformerID, limit uint) ([]Deployment, error) {
	span, ctx := tracer.StartSpanFromContext(ctx, "DBSelectDeploymentsByTransformerID")
	defer span.Finish()
	selectQuery := h.AdaptQuery(`
		SELECT created, releaseVersion, appName, envName, metadata, transformereslVersion
		FROM deployments
		WHERE transformereslVersion=?
		LIMIT ?;
	`)

	span.SetTag("query", selectQuery)
	rows, err := tx.QueryContext(
		ctx,
		selectQuery,
		transformerID,
		limit,
	)
	if err != nil {
		return nil, fmt.Errorf("could not select deployments by transformer id from DB. Error: %w\n", err)
	}
	defer func(rows *sql.Rows) {
		err := rows.Close()
		if err != nil {
			logger.FromContext(ctx).Sugar().Warnf("deployments: row closing error: %v", err)
		}
	}(rows)
	deployments := make([]Deployment, 0)
	for rows.Next() {
		row, err := h.processSingleDeploymentRow(ctx, rows)
		if err != nil {
			return nil, err
		}
		deployments = append(deployments, *row)
	}
	err = closeRows(rows)
	if err != nil {
		return nil, err
	}
	return deployments, nil
}

func (h *DBHandler) DBSelectAnyDeployment(ctx context.Context, tx *sql.Tx) (*DBDeployment, error) {
	span, ctx := tracer.StartSpanFromContext(ctx, "DBSelectAnyDeployment")
	defer span.Finish()
	selectQuery := h.AdaptQuery(`
		SELECT created, releaseVersion, appName, envName
		FROM deployments
		LIMIT 1;
	`)
	span.SetTag("query", selectQuery)

	rows, err := tx.QueryContext(
		ctx,
		selectQuery,
	)
	if err != nil {
		return nil, fmt.Errorf("could not select any deployments from DB. Error: %w\n", err)
	}
	defer func(rows *sql.Rows) {
		err := rows.Close()
		if err != nil {
			logger.FromContext(ctx).Sugar().Warnf("deployments row could not be closed: %v", err)
		}
	}(rows)
	//exhaustruct:ignore
	var row = &DBDeployment{}
	if rows.Next() {
		var releaseVersion sql.NullInt64
		err := rows.Scan(&row.Created, &releaseVersion, &row.App, &row.Env)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return nil, nil
			}
			return nil, fmt.Errorf("Error scanning deployments row from DB. Error: %w\n", err)
		}
		if releaseVersion.Valid {
			row.ReleaseVersion = &releaseVersion.Int64
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

func (h *DBHandler) DBSelectAllDeploymentsForApp(ctx context.Context, tx *sql.Tx, appName string) (map[string]int64, error) {
	span, ctx := tracer.StartSpanFromContext(ctx, "DBSelectAllDeploymentsForApp")
	defer span.Finish()
	insertQuery := h.AdaptQuery(`
		SELECT envname, releaseVersion
		FROM deployments
		WHERE appName = (?) AND releaseVersion IS NOT NULL;
	`)
	if h == nil {
		return nil, nil
	}
	if tx == nil {
		return nil, fmt.Errorf("DBSelectAllDeploymentsForApp: no transaction provided")
	}

	span.SetTag("query", insertQuery)
	rows, err := tx.Query(
		insertQuery,
		appName,
	)

	return h.processAllDeploymentRow(ctx, err, rows)
}

func (h *DBHandler) DBSelectAllDeploymentsForAppAtTimestamp(ctx context.Context, tx *sql.Tx, appName string, ts time.Time) (map[string]int64, error) {
	span, ctx := tracer.StartSpanFromContext(ctx, "DBSelectAllDeploymentsForAppAtTimestamp")
	defer span.Finish()
	query := h.AdaptQuery(`
	SELECT
		deployments_history.envName,
		deployments_history.releaseVersion
	FROM (
	SELECT
		MAX(version) AS latest,
		appname,
		envname
	FROM
		deployments_history
	WHERE deployments_history.appname = (?) AND created <= (?) AND deployments_history.releaseVersion IS NOT NULL
	GROUP BY
		envName, appname
	) AS latest
	JOIN
		deployments_history AS deployments_history
	ON
		latest.latest=deployments_history.version
		AND latest.appname=deployments_history.appname
		AND latest.envName=deployments_history.envName;`)
	if h == nil {
		return nil, nil
	}
	if tx == nil {
		return nil, fmt.Errorf("DBSelectAllDeploymentsForAppAtTimestamp: no transaction provided")
	}

	span.SetTag("query", query)
	rows, err := tx.Query(
		query,
		appName,
		ts,
	)

	return h.processAllDeploymentRow(ctx, err, rows)
}

// UPDATE, DELETE, INSERT

func (h *DBHandler) DBUpdateOrCreateDeployment(ctx context.Context, tx *sql.Tx, deployment Deployment) error {
	span, ctx := tracer.StartSpanFromContext(ctx, "DBUpdateOrCreateDeployment")
	defer span.Finish()
	err := h.upsertDeploymentRow(ctx, tx, deployment)
	if err != nil {
		return err
	}
	err = h.insertDeploymentHistoryRow(ctx, tx, deployment)
	if err != nil {
		return err
	}
	return nil
}

// Internal functions

func (h *DBHandler) upsertDeploymentRow(ctx context.Context, tx *sql.Tx, deployment Deployment) error {
	span, ctx := tracer.StartSpanFromContext(ctx, "upsertDeploymentRow")
	defer span.Finish()
	upsertQuery := h.AdaptQuery(`
		INSERT INTO deployments (created, releaseVersion, appName, envName, metadata, transformereslVersion)
		VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT(appName, envName)
		DO UPDATE SET created = excluded.created, releaseVersion = excluded.releaseVersion, metadata = excluded.metadata, transformereslversion = excluded.transformereslversion;
	`)
	if h == nil {
		return nil
	}
	if tx == nil {
		return fmt.Errorf("upsertDeploymentRow: no transaction provided")
	}

	jsonToInsert, err := json.Marshal(deployment.Metadata)
	if err != nil {
		return fmt.Errorf("could not marshal json data: %w", err)
	}

	now, err := h.DBReadTransactionTimestamp(ctx, tx)
	if err != nil {
		return fmt.Errorf("upsertDeploymnetRow unable to get transaction timestamp: %w", err)
	}
	span.SetTag("query", upsertQuery)
	nullVersion := NewNullInt(deployment.Version)

	_, err = tx.Exec(
		upsertQuery,
		*now,
		nullVersion,
		deployment.App,
		deployment.Env,
		jsonToInsert,
		deployment.TransformerID)

	if err != nil {
		return fmt.Errorf("could not write deployment into DB. Error: %w\n", err)
	}
	return nil
}

func (h *DBHandler) insertDeploymentHistoryRow(ctx context.Context, tx *sql.Tx, deployment Deployment) error {
	span, ctx := tracer.StartSpanFromContext(ctx, "insertDeploymentHistoryRow")
	defer span.Finish()
	insertQuery := h.AdaptQuery(`
		INSERT INTO deployments_history (created, releaseVersion, appName, envName, metadata, transformereslVersion) 
		VALUES (?, ?, ?, ?, ?, ?);
	`)
	if h == nil {
		return nil
	}
	if tx == nil {
		return fmt.Errorf("DBWriteDeployment: no transaction provided")
	}

	jsonToInsert, err := json.Marshal(deployment.Metadata)
	if err != nil {
		return fmt.Errorf("could not marshal json data: %w", err)
	}

	now, err := h.DBReadTransactionTimestamp(ctx, tx)
	if err != nil {
		return fmt.Errorf("DBWriteDeployment unable to get transaction timestamp: %w", err)
	}
	span.SetTag("query", insertQuery)
	nullVersion := NewNullInt(deployment.Version)

	_, err = tx.Exec(
		insertQuery,
		*now,
		nullVersion,
		deployment.App,
		deployment.Env,
		jsonToInsert,
		deployment.TransformerID)

	if err != nil {
		return fmt.Errorf("could not write deployment_history into DB. Error: %w\n", err)
	}
	return nil
}

// process Rows

func processDeployment(rows *sql.Rows) (*Deployment, error) {
	var releaseVersion sql.NullInt64
	var row = &DBDeployment{
		Created:        time.Time{},
		ReleaseVersion: nil,
		App:            "",
		Env:            "",
		Metadata:       "",
		TransformerID:  0,
	}
	//exhaustruct:ignore
	var resultJson = DeploymentMetadata{}
	if rows.Next() {
		err := rows.Scan(&row.Created, &releaseVersion, &row.App, &row.Env, &row.Metadata, &row.TransformerID)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return nil, nil
			}
			return nil, fmt.Errorf("Error scanning deployments row from DB. Error: %w\n", err)
		}
		if releaseVersion.Valid {
			row.ReleaseVersion = &releaseVersion.Int64
		}

		err = json.Unmarshal(([]byte)(row.Metadata), &resultJson)
		if err != nil {
			return nil, fmt.Errorf("Error during json unmarshal in deployments. Error: %w. Data: %s\n", err, row.Metadata)
		}
	}
	err := rows.Close()
	if err != nil {
		return nil, fmt.Errorf("deployments: row closing error: %v\n", err)
	}
	err = rows.Err()
	if err != nil {
		return nil, fmt.Errorf("deployments: row has error: %v\n", err)
	}
	return &Deployment{
		Created:       row.Created,
		App:           row.App,
		Env:           row.Env,
		Version:       row.ReleaseVersion,
		Metadata:      resultJson,
		TransformerID: row.TransformerID,
	}, nil
}

func processAllLatestDeploymentsForApp(rows *sql.Rows) (map[string]Deployment, error) {
	result := make(map[string]Deployment)
	for rows.Next() {
		var curr = Deployment{
			Created: time.Time{},
			Env:     "",
			App:     "",
			Version: nil,
			Metadata: DeploymentMetadata{
				DeployedByName:  "",
				DeployedByEmail: "",
				CiLink:          "",
			},
			TransformerID: 0,
		}
		var releaseVersion sql.NullInt64
		var jsonMetadata string
		err := rows.Scan(&curr.Created, &curr.App, &releaseVersion, &curr.Env, &jsonMetadata)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return nil, nil
			}
			return nil, fmt.Errorf("Error scanning deployments row from DB. Error: %w\n", err)
		}
		err = json.Unmarshal(([]byte)(jsonMetadata), &curr.Metadata)
		if err != nil {
			return nil, fmt.Errorf("Error during json unmarshal in deployments. Error: %w. Data: %s\n", err, jsonMetadata)
		}
		if releaseVersion.Valid {
			curr.Version = &releaseVersion.Int64
		}
		result[curr.Env] = curr
	}
	err := rows.Close()
	if err != nil {
		return nil, fmt.Errorf("deployments: row closing error: %v\n", err)
	}
	err = rows.Err()
	if err != nil {
		return nil, fmt.Errorf("deployments: row has error: %v\n", err)
	}
	return result, nil
}

func processAllLatestDeployments(rows *sql.Rows) (map[string]*int64, error) {
	result := make(map[string]*int64)
	for rows.Next() {
		var releaseVersion sql.NullInt64
		var appName string
		err := rows.Scan(&appName, &releaseVersion)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return nil, nil
			}
			return nil, fmt.Errorf("Error scanning deployments row from DB. Error: %w\n", err)
		}

		if releaseVersion.Valid {
			result[appName] = &releaseVersion.Int64
		}
	}
	err := rows.Close()
	if err != nil {
		return nil, fmt.Errorf("deployments: row closing error: %v\n", err)
	}
	err = rows.Err()
	if err != nil {
		return nil, fmt.Errorf("deployments: row has error: %v\n", err)
	}
	return result, nil
}

func (h *DBHandler) processSingleDeploymentRow(ctx context.Context, rows *sql.Rows) (*Deployment, error) {
	var row = &DBDeployment{
		Created:        time.Time{},
		ReleaseVersion: nil,
		App:            "",
		Env:            "",
		Metadata:       "",
		TransformerID:  0,
	}
	var releaseVersion sql.NullInt64
	//exhaustruct:ignore
	var resultJson = DeploymentMetadata{}

	err := rows.Scan(&row.Created, &releaseVersion, &row.App, &row.Env, &row.Metadata, &row.TransformerID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("Error scanning deployments row from DB. Error: %w\n", err)
	}
	if releaseVersion.Valid {
		row.ReleaseVersion = &releaseVersion.Int64
	}

	err = json.Unmarshal(([]byte)(row.Metadata), &resultJson)
	if err != nil {
		return nil, fmt.Errorf("Error during json unmarshal in deployments. Error: %w. Data: %s\n", err, row.Metadata)
	}

	return &Deployment{
		Created:       row.Created,
		App:           row.App,
		Env:           row.Env,
		Version:       row.ReleaseVersion,
		Metadata:      resultJson,
		TransformerID: row.TransformerID,
	}, nil
}

func (h *DBHandler) processAllDeploymentRow(ctx context.Context, err error, rows *sql.Rows) (map[string]int64, error) {
	if err != nil {
		return nil, fmt.Errorf("could not query deployments table from DB. Error: %w\n", err)
	}
	defer func(rows *sql.Rows) {
		err := rows.Close()
		if err != nil {
			logger.FromContext(ctx).Sugar().Warnf("deployments: row could not be closed: %v", err)
		}
	}(rows)
	deployments := make(map[string]int64)
	for rows.Next() {
		var rowVersion int64
		var rowEnv string
		err := rows.Scan(&rowEnv, &rowVersion)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return nil, nil
			}
			return nil, fmt.Errorf("Error scanning oldest_deployments row from DB. Error: %w\n", err)
		}
		deployments[rowEnv] = rowVersion
	}
	err = closeRows(rows)
	if err != nil {
		return nil, err
	}
	return deployments, nil
}
