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

	"github.com/freiheit-com/kuberpult/pkg/tracing"
	"gopkg.in/DataDog/dd-trace-go.v1/ddtrace/tracer"
)

type DBDeployment struct {
	Created        time.Time
	ReleaseVersion *uint64
	App            string
	Env            string
	TransformerID  TransformerID
	Revision       uint64
	Metadata       string // json
}

type DeploymentMetadata struct {
	DeployedByName  string
	DeployedByEmail string
	CiLink          string
}

type Deployment struct {
	Created        time.Time
	App            string
	Env            types.EnvName
	ReleaseNumbers types.ReleaseNumbers
	Metadata       DeploymentMetadata
	TransformerID  TransformerID
}

type QueuedDeployment struct {
	Created        time.Time
	Env            types.EnvName
	App            string
	ReleaseNumbers types.ReleaseNumbers
}

// SELECT

func (h *DBHandler) DBSelectLatestDeployment(ctx context.Context, tx *sql.Tx, appSelector string, envSelector types.EnvName) (*Deployment, error) {
	span, ctx := tracer.StartSpanFromContext(ctx, "DBSelectLatestDeployment")
	defer span.Finish()

	selectQuery := h.AdaptQuery(`
		SELECT created, releaseVersion, appName, envName, metadata, transformereslVersion, revision
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
		return nil, fmt.Errorf("could not select deployment for app %s on env %s from DB. Error: %w", appSelector, envSelector, err)
	}
	defer closeRowsAndLog(rows, ctx, "DBSelectLatestDeployment")
	return processDeployment(rows)
}

func (h *DBHandler) DBSelectLatestDeploymentAtTimestamp(ctx context.Context, tx *sql.Tx, appSelector string, envSelector types.EnvName, ts time.Time) (*Deployment, error) {

	selectQuery := h.AdaptQuery(`
		SELECT created, releaseVersion, appName, envName, metadata, transformereslVersion, revision
		FROM deployments_history
		WHERE appName=? AND envName=? AND created <=?
		ORDER BY version DESC
		LIMIT 1;
	`)
	rows, err := tx.QueryContext(
		ctx,
		selectQuery,
		appSelector,
		envSelector,
		ts,
	)
	if err != nil {
		return nil, fmt.Errorf("could not select deployment for app %s on env %s from DB. Error: %w", appSelector, envSelector, err)
	}
	defer closeRowsAndLog(rows, ctx, "DBSelectLatestDeploymentAtTimestamp")
	return processDeployment(rows)
}

func (h *DBHandler) DBSelectAllLatestDeploymentsForApplication(ctx context.Context, tx *sql.Tx, appName string) (map[types.EnvName]Deployment, error) {
	span, ctx := tracer.StartSpanFromContext(ctx, "DBSelectAllLatestDeploymentsForApplication")
	defer span.Finish()

	selectQuery := h.AdaptQuery(`
		SELECT created, appname, releaseVersion, envName, metadata, transformereslversion, revision
		FROM deployments
		WHERE deployments.appname = (?) AND deployments.releaseVersion IS NOT NULL
		ORDER BY envName;
	`)
	span.SetTag("query", selectQuery)
	rows, err := tx.QueryContext(
		ctx,
		selectQuery,
		appName,
	)
	if err != nil {
		return nil, fmt.Errorf("could not select deployment of app %s from DB. Error: %w", appName, err)
	}
	defer closeRowsAndLog(rows, ctx, "DBSelectAllLatestDeploymentsForApplication")
	return processAllLatestDeploymentsForApp(rows)
}

func (h *DBHandler) DBSelectOldestDeploymentForApplication(ctx context.Context, tx *sql.Tx, appName string) (*Deployment, error) {
	span, ctx := tracer.StartSpanFromContext(ctx, "DBSelectOldestDeploymentForApplication")
	defer span.Finish()

	selectQuery := h.AdaptQuery(`
		SELECT created, releaseVersion, appName, envName, metadata, transformereslVersion, revision
		FROM deployments
		WHERE deployments.appname = (?) AND deployments.releaseVersion IS NOT NULL
		ORDER BY releaseVersion ASC, revision ASC
		LIMIT 1;
	`)
	span.SetTag("query", selectQuery)
	rows, err := tx.QueryContext(
		ctx,
		selectQuery,
		appName,
	)
	if err != nil {
		return nil, fmt.Errorf("could not select deployment of app %s from DB. Error: %w", appName, err)
	}
	defer closeRowsAndLog(rows, ctx, "DBSelectOldestDeploymentForApplication")
	return processDeployment(rows)
}

func (h *DBHandler) DBSelectAllLatestDeploymentsOnEnvironment(ctx context.Context, tx *sql.Tx, envName types.EnvName) (map[string]types.ReleaseNumbers, error) {
	span, ctx := tracer.StartSpanFromContext(ctx, "DBSelectAllLatestDeploymentsOnEnvironment")
	defer span.Finish()

	selectQuery := h.AdaptQuery(`
		SELECT appName, releaseVersion, revision
		FROM deployments
		WHERE deployments.envName= ?
		ORDER BY appName;
	`)

	span.SetTag("query", selectQuery)
	span.SetTag("kuberpultEnvironment", envName)
	rows, err := tx.QueryContext(
		ctx,
		selectQuery,
		envName,
	)
	if err != nil {
		return nil, fmt.Errorf("could not select deployment for env %s from DB. Error: %w", envName, err)
	}
	defer closeRowsAndLog(rows, ctx, "DBSelectAllLatestDeploymentsOnEnvironment")
	return processAllLatestDeployments(rows)
}

func (h *DBHandler) DBSelectSpecificDeployment(ctx context.Context, tx *sql.Tx, appSelector string, envSelector string, releaseVersion uint64) (*Deployment, error) {
	span, ctx := tracer.StartSpanFromContext(ctx, "DBSelectSpecificDeployment")
	defer span.Finish()

	selectQuery := h.AdaptQuery(`
		SELECT created, releaseVersion, appName, envName, metadata, transformereslVersion, revision
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
		return nil, fmt.Errorf("could not select deployment for app %s on env %s for version %v from DB. Error: %w", appSelector, envSelector, releaseVersion, err)
	}
	defer closeRowsAndLog(rows, ctx, "DBSelectSpecificDeployment")
	return processDeployment(rows)
}

func (h *DBHandler) DBSelectSpecificDeploymentHistory(ctx context.Context, tx *sql.Tx, appSelector string, envSelector string, releaseVersion uint64) (*Deployment, error) {
	span, ctx := tracer.StartSpanFromContext(ctx, "DBSelectSpecificDeploymentHistory")
	defer span.Finish()

	selectQuery := h.AdaptQuery(`
		SELECT created, releaseVersion, appName, envName, metadata, transformereslVersion, revision
		FROM deployments_history
		WHERE appName=? AND envName=? and releaseVersion=?
		ORDER BY created DESC
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
		return nil, fmt.Errorf("could not select deployment for app %s on env %s for version %v from DB. Error: %w", appSelector, envSelector, releaseVersion, err)
	}
	defer closeRowsAndLog(rows, ctx, "DBSelectSpecificDeploymentHistory")
	return processDeployment(rows)
}

func (h *DBHandler) DBSelectDeploymentHistory(ctx context.Context, tx *sql.Tx, appSelector string, envSelector string, limit int) ([]Deployment, error) {
	span, ctx := tracer.StartSpanFromContext(ctx, "DBSelectDeploymentHistory")
	defer span.Finish()

	selectQuery := h.AdaptQuery(`
		SELECT created, releaseVersion, appName, envname, metadata, transformereslversion, revision
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
		return nil, fmt.Errorf("could not select deployment history of app %s in env %s from DB. Error: %w", appSelector, envSelector, err)
	}
	defer closeRowsAndLog(rows, ctx, "DBSelectDeploymentHistory")

	result := make([]Deployment, 0)

	for rows.Next() {
		row, err := h.processSingleDeploymentRow(ctx, rows)
		if err != nil {
			return nil, err
		}
		result = append(result, *row)
	}
	return result, nil
}

func (h *DBHandler) DBSelectDeploymentHistoryCount(ctx context.Context, tx *sql.Tx, envSelector string, startDate time.Time, endDate time.Time) (uint64, error) {
	span, ctx, onErr := tracing.StartSpanFromContext(ctx, "DBSelectDeploymentHistoryCount")
	defer span.Finish()

	selectQuery := h.AdaptQuery(`
		SELECT COUNT(*) FROM deployments_history
		WHERE releaseversion IS NOT NULL AND created >= (?) AND created <= (?) AND envname = (?);
	`)

	span.SetTag("query", selectQuery)
	var result uint64
	err := tx.QueryRowContext(
		ctx,
		selectQuery,
		startDate,
		endDate,
		envSelector,
	).Scan(&result)

	if err != nil {
		return 0, onErr(err)
	}

	return result, nil
}

func (h *DBHandler) DBSelectDeploymentsByTransformerID(ctx context.Context, tx *sql.Tx, transformerID TransformerID) ([]Deployment, error) {
	span, ctx := tracer.StartSpanFromContext(ctx, "DBSelectDeploymentsByTransformerID")
	defer span.Finish()

	selectQuery := h.AdaptQuery(`
		SELECT created, releaseVersion, appName, envName, metadata, transformereslVersion, revision
		FROM deployments
		WHERE transformereslVersion=?;
	`)

	span.SetTag("query", selectQuery)
	rows, err := tx.QueryContext(
		ctx,
		selectQuery,
		transformerID,
	)
	if err != nil {
		return nil, fmt.Errorf("could not select deployments by transformer id from DB. Error: %w", err)
	}
	defer closeRowsAndLog(rows, ctx, "DBSelectDeploymentsByTransformerID")
	deployments := make([]Deployment, 0)
	for rows.Next() {
		row, err := h.processSingleDeploymentRow(ctx, rows)
		if err != nil {
			return nil, err
		}
		deployments = append(deployments, *row)
	}
	return deployments, nil
}

func (h *DBHandler) DBHasAnyDeployment(ctx context.Context, tx *sql.Tx) (bool, error) {
	span, ctx := tracer.StartSpanFromContext(ctx, "DBHasAnyDeployment")
	defer span.Finish()

	selectQuery := h.AdaptQuery(`
		SELECT created, releaseVersion, appName, envName, revision
		FROM deployments
		LIMIT 1;
	`)
	span.SetTag("query", selectQuery)

	rows, err := tx.QueryContext(
		ctx,
		selectQuery,
	)
	if err != nil {
		return false, fmt.Errorf("could not select any deployments from DB. Error: %w", err)
	}
	defer closeRowsAndLog(rows, ctx, "DBHasAnyDeployment")
	return rows.Next(), nil
}

func (h *DBHandler) DBSelectAllDeploymentsForApp(ctx context.Context, tx *sql.Tx, appName string) (map[types.EnvName]types.ReleaseNumbers, error) {
	span, ctx := tracer.StartSpanFromContext(ctx, "DBSelectAllDeploymentsForApp")
	defer span.Finish()

	insertQuery := h.AdaptQuery(`
		SELECT envName, releaseVersion, revision
		FROM deployments
		WHERE appName = (?) AND releaseVersion IS NOT NULL
		ORDER BY envName;
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

func (h *DBHandler) DBSelectAllDeploymentsForAppAtTimestamp(ctx context.Context, tx *sql.Tx, appName string, ts time.Time) (map[types.EnvName]types.ReleaseNumbers, error) {
	span, ctx := tracer.StartSpanFromContext(ctx, "DBSelectAllDeploymentsForAppAtTimestamp")
	defer span.Finish()

	query := h.AdaptQuery(`
	SELECT
		deployments_history.envName,
		deployments_history.releaseVersion,
	    deployments_history.revision
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
	span.SetTag("query", query)

	rows, err := tx.Query(
		query,
		appName,
		ts,
	)

	return h.processAllDeploymentRow(ctx, err, rows)
}

func (h *DBHandler) DBSelectDeploymentAttemptHistory(ctx context.Context, tx *sql.Tx, environmentName types.EnvName, appName string, limit int) ([]QueuedDeployment, error) {
	span, ctx := tracer.StartSpanFromContext(ctx, "DBSelectDeploymentAttemptHistory")
	defer span.Finish()

	query := h.AdaptQuery(
		"SELECT created, envName, appName, releaseVersion, revision FROM deployment_attempts_history WHERE envName=? AND appName=? ORDER BY eslId DESC LIMIT ?;")

	span.SetTag("query", query)
	rows, err := tx.QueryContext(
		ctx,
		query,
		environmentName,
		appName, limit)

	if err != nil {
		return nil, fmt.Errorf("could not query deployment attempts table from DB. Error: %w", err)
	}
	defer closeRowsAndLog(rows, ctx, "DBSelectDeploymentAttemptHistory")
	queuedDeployments := make([]QueuedDeployment, 0)
	for rows.Next() {
		//exhaustruct:ignore
		var deployment = QueuedDeployment{}
		var releaseVersion sql.NullInt64
		err := rows.Scan(&deployment.Created, &deployment.Env, &deployment.App, &releaseVersion, &deployment.ReleaseNumbers.Revision)
		if err != nil {
			return nil, fmt.Errorf("error scanning deployment attempts row from DB. Error: %w", err)
		}
		if releaseVersion.Valid { // sooo, if we deleted an attempt, releaseVersion is NULL and we just skip parsing it here.
			conv := uint64(releaseVersion.Int64)
			deployment.ReleaseNumbers.Version = &conv
		}
		queuedDeployments = append(queuedDeployments, deployment)
	}
	return queuedDeployments, nil
}

func (h *DBHandler) DBSelectLatestDeploymentAttemptOfAllApps(ctx context.Context, tx *sql.Tx, environmentName types.EnvName) ([]*QueuedDeployment, error) {
	span, ctx := tracer.StartSpanFromContext(ctx, "DBSelectLatestDeploymentAttemptOfAllApps")
	defer span.Finish()

	query := h.AdaptQuery(
		`
SELECT created, envName, appName, releaseVersion, revision
FROM deployment_attempts_latest
WHERE envName=?
ORDER BY appName;
		`)
	span.SetTag("query", query)
	rows, err := tx.QueryContext(
		ctx,
		query,
		environmentName)
	return h.processDeploymentAttemptsRows(ctx, rows, err)
}

func (h *DBHandler) DBSelectLatestDeploymentAttemptOnAllEnvironments(ctx context.Context, tx *sql.Tx, appName string) ([]*QueuedDeployment, error) {
	span, ctx := tracer.StartSpanFromContext(ctx, "DBSelectLatestDeploymentAttemptOnAllEnvironments")
	defer span.Finish()

	query := h.AdaptQuery(
		`
SELECT created, envName, appName, releaseVersion, revision
FROM deployment_attempts_latest
WHERE appName=?
ORDER BY envName;
		`)
	span.SetTag("query", query)
	rows, err := tx.QueryContext(
		ctx,
		query,
		appName)
	return h.processDeploymentAttemptsRows(ctx, rows, err)
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

func (h *DBHandler) DBWriteDeploymentAttempt(ctx context.Context, tx *sql.Tx, envName types.EnvName, appName string, version types.ReleaseNumbers) error {
	span, ctx := tracer.StartSpanFromContext(ctx, "DBWriteDeploymentAttempt")
	defer span.Finish()

	return h.dbWriteDeploymentAttemptInternal(ctx, tx, &QueuedDeployment{
		Created:        time.Time{},
		Env:            envName,
		App:            appName,
		ReleaseNumbers: version,
	})
}

func (h *DBHandler) DBDeleteDeploymentAttempt(ctx context.Context, tx *sql.Tx, envName types.EnvName, appName string) error {
	span, ctx := tracer.StartSpanFromContext(ctx, "DBDeleteDeploymentAttempt")
	defer span.Finish()

	return h.dbWriteDeploymentAttemptInternal(ctx, tx, &QueuedDeployment{
		Created: time.Time{},
		Env:     envName,
		App:     appName,
		ReleaseNumbers: types.ReleaseNumbers{
			Version:  nil,
			Revision: 0,
		},
	})
}
func (h *DBHandler) DBMigrationUpdateDeploymentsTimestamp(ctx context.Context, transaction *sql.Tx, application string, releaseversion uint64, env types.EnvName, createdAt time.Time, revision uint64) error {
	span, ctx, onErr := tracing.StartSpanFromContext(ctx, "DBMigrationUpdateDeploymentsTimestamp")
	defer span.Finish()
	historyUpdateQuery := h.AdaptQuery(`
		UPDATE deployments_history SET created=? WHERE appname=? AND releaseversion=? AND envname=? AND revision=?;
	`)

	_, err := transaction.ExecContext(
		ctx,
		historyUpdateQuery,
		createdAt,
		application,
		releaseversion,
		string(env),
		revision,
	)
	if err != nil {
		return onErr(fmt.Errorf(
			"could not update deployments_history timestamp for app '%s' and env '%s' and version '%v' into DB. Error: %w",
			application,
			env,
			releaseversion,
			err))
	}

	deploymentsUpdateQuery := h.AdaptQuery(`
		UPDATE deployments SET created=? WHERE appname=? AND releaseversion=? AND envname=? AND revision=?;
	`)

	_, err = transaction.ExecContext(
		ctx,
		deploymentsUpdateQuery,
		createdAt,
		application,
		releaseversion,
		string(env),
		revision,
	)
	if err != nil {
		return onErr(fmt.Errorf(
			"could not update releases timestamp for app '%s' and version '%v' into DB. Error: %w",
			application,
			releaseversion,
			err))
	}
	return nil

}

// Internal functions

func (h *DBHandler) upsertDeploymentRow(ctx context.Context, tx *sql.Tx, deployment Deployment) error {
	span, ctx := tracer.StartSpanFromContext(ctx, "upsertDeploymentRow")
	defer span.Finish()

	upsertQuery := h.AdaptQuery(`
		INSERT INTO deployments (created, releaseVersion, appName, envName, metadata, transformereslVersion, revision)
		VALUES (?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(appName, envName)
		DO UPDATE SET created = excluded.created, releaseVersion = excluded.releaseVersion, metadata = excluded.metadata, transformereslversion = excluded.transformereslversion, revision = excluded.revision;
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
	nullVersion := NewNullUInt(deployment.ReleaseNumbers.Version)

	_, err = tx.Exec(
		upsertQuery,
		*now,
		nullVersion,
		deployment.App,
		deployment.Env,
		jsonToInsert,
		deployment.TransformerID,
		deployment.ReleaseNumbers.Revision)

	if err != nil {
		return fmt.Errorf("could not write deployment into DB. Error: %w", err)
	}
	return nil
}

func (h *DBHandler) insertDeploymentHistoryRow(ctx context.Context, tx *sql.Tx, deployment Deployment) error {
	span, ctx := tracer.StartSpanFromContext(ctx, "insertDeploymentHistoryRow")
	defer span.Finish()

	insertQuery := h.AdaptQuery(`
		INSERT INTO deployments_history (created, releaseVersion, appName, envName, metadata, transformereslVersion, revision) 
		VALUES (?, ?, ?, ?, ?, ?, ?);
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
	nullVersion := NewNullUInt(deployment.ReleaseNumbers.Version)

	_, err = tx.Exec(
		insertQuery,
		*now,
		nullVersion,
		deployment.App,
		deployment.Env,
		jsonToInsert,
		deployment.TransformerID,
		deployment.ReleaseNumbers.Revision,
	)

	if err != nil {
		return fmt.Errorf("could not write deployment_history into DB. Error: %w", err)
	}
	return nil
}

// process Rows
func processDeployment(rows *sql.Rows) (*Deployment, error) {
	var releaseVersion sql.NullInt64

	var toReturn *Deployment
	//exhaustruct:ignore
	var resultJson = DeploymentMetadata{}
	if rows.Next() {
		var row = &DBDeployment{
			Created:        time.Time{},
			ReleaseVersion: nil,
			App:            "",
			Env:            "",
			Metadata:       "",
			TransformerID:  0,
			Revision:       0,
		}
		err := rows.Scan(&row.Created, &releaseVersion, &row.App, &row.Env, &row.Metadata, &row.TransformerID, &row.Revision)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return nil, nil
			}
			return nil, fmt.Errorf("error scanning deployments row from DB. Error: %w", err)
		}
		if releaseVersion.Valid {
			conv := uint64(releaseVersion.Int64)
			row.ReleaseVersion = &conv
		}

		err = json.Unmarshal(([]byte)(row.Metadata), &resultJson)
		if err != nil {
			return nil, fmt.Errorf("error during json unmarshal in deployments. Error: %w. Data: %s", err, row.Metadata)
		}
		toReturn = &Deployment{
			Created: row.Created,
			App:     row.App,
			Env:     types.EnvName(row.Env),
			ReleaseNumbers: types.ReleaseNumbers{
				Revision: row.Revision,
				Version:  row.ReleaseVersion,
			},
			Metadata:      resultJson,
			TransformerID: row.TransformerID,
		}
	} else {
		toReturn = nil
	}
	err := rows.Close()
	if err != nil {
		return nil, fmt.Errorf("deployments: row closing error: %v", err)
	}
	err = rows.Err()
	if err != nil {
		return nil, fmt.Errorf("deployments: row has error: %v", err)
	}
	return toReturn, nil
}

func processAllLatestDeploymentsForApp(rows *sql.Rows) (map[types.EnvName]Deployment, error) {
	result := make(map[types.EnvName]Deployment)
	for rows.Next() {
		var curr = Deployment{
			Created: time.Time{},
			Env:     "",
			App:     "",
			ReleaseNumbers: types.ReleaseNumbers{
				Revision: 0,
				Version:  nil,
			},
			Metadata: DeploymentMetadata{
				DeployedByName:  "",
				DeployedByEmail: "",
				CiLink:          "",
			},
			TransformerID: 0,
		}
		var releaseVersion sql.NullInt64
		var jsonMetadata string
		err := rows.Scan(&curr.Created, &curr.App, &releaseVersion, &curr.Env, &jsonMetadata, &curr.TransformerID, &curr.ReleaseNumbers.Revision)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return nil, nil
			}
			return nil, fmt.Errorf("error scanning deployments row from DB. Error: %w", err)
		}
		err = json.Unmarshal(([]byte)(jsonMetadata), &curr.Metadata)
		if err != nil {
			return nil, fmt.Errorf("error during json unmarshal in deployments. Error: %w. Data: %s", err, jsonMetadata)
		}
		if releaseVersion.Valid {
			conv := uint64(releaseVersion.Int64)
			curr.ReleaseNumbers.Version = &conv
		}
		result[curr.Env] = curr
	}
	err := rows.Close()
	if err != nil {
		return nil, fmt.Errorf("deployments: row closing error: %v", err)
	}
	err = rows.Err()
	if err != nil {
		return nil, fmt.Errorf("deployments: row has error: %v", err)
	}
	return result, nil
}

func processAllLatestDeployments(rows *sql.Rows) (map[string]types.ReleaseNumbers, error) {
	result := make(map[string]types.ReleaseNumbers)
	for rows.Next() {
		var releaseVersion sql.NullInt64
		var appName string
		var revision uint64
		err := rows.Scan(&appName, &releaseVersion, &revision)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return nil, nil
			}
			return nil, fmt.Errorf("error scanning deployments row from DB. Error: %w", err)
		}

		if releaseVersion.Valid {
			v := uint64(releaseVersion.Int64)
			result[appName] = types.ReleaseNumbers{
				Version:  &v,
				Revision: revision,
			}
		}
	}
	err := rows.Close()
	if err != nil {
		return nil, fmt.Errorf("deployments: row closing error: %v", err)
	}
	err = rows.Err()
	if err != nil {
		return nil, fmt.Errorf("deployments: row has error: %v", err)
	}
	return result, nil
}

func (h *DBHandler) processSingleDeploymentRow(_ context.Context, rows *sql.Rows) (*Deployment, error) {
	var row = &DBDeployment{
		Created:        time.Time{},
		ReleaseVersion: nil,
		App:            "",
		Env:            "",
		Metadata:       "",
		TransformerID:  0,
		Revision:       0,
	}
	var releaseVersion sql.NullInt64
	//exhaustruct:ignore
	var resultJson = DeploymentMetadata{}

	err := rows.Scan(&row.Created, &releaseVersion, &row.App, &row.Env, &row.Metadata, &row.TransformerID, &row.Revision)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("error scanning deployments row from DB. Error: %w", err)
	}

	if releaseVersion.Valid {
		conv := uint64(releaseVersion.Int64)
		row.ReleaseVersion = &conv
	}

	err = json.Unmarshal(([]byte)(row.Metadata), &resultJson)
	if err != nil {
		return nil, fmt.Errorf("error during json unmarshal in deployments. Error: %w. Data: %s", err, row.Metadata)
	}

	return &Deployment{
		Created: row.Created,
		App:     row.App,
		Env:     types.EnvName(row.Env),
		ReleaseNumbers: types.ReleaseNumbers{
			Revision: row.Revision,
			Version:  row.ReleaseVersion,
		},
		Metadata:      resultJson,
		TransformerID: row.TransformerID,
	}, nil
}

func (h *DBHandler) processAllDeploymentRow(ctx context.Context, err error, rows *sql.Rows) (map[types.EnvName]types.ReleaseNumbers, error) {
	if err != nil {
		return nil, fmt.Errorf("could not query deployments table from DB. Error: %w", err)
	}
	defer closeRowsAndLog(rows, ctx, "deployment")
	deployments := make(map[types.EnvName]types.ReleaseNumbers)
	for rows.Next() {
		var rowVersion types.ReleaseNumbers
		var rowEnv types.EnvName
		err := rows.Scan(&rowEnv, &rowVersion.Version, &rowVersion.Revision)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return nil, nil
			}
			return nil, fmt.Errorf("error scanning oldest_deployments row from DB. Error: %w", err)
		}
		deployments[rowEnv] = rowVersion
	}
	err = closeRows(rows)
	if err != nil {
		return nil, err
	}
	return deployments, nil
}

func (h *DBHandler) processDeploymentAttemptsRows(ctx context.Context, rows *sql.Rows, err error) ([]*QueuedDeployment, error) {
	if err != nil {
		return nil, fmt.Errorf("error in executing query: %w", err)
	}
	defer closeRowsAndLog(rows, ctx, "deployment attempts")
	result := []*QueuedDeployment{}
	for rows.Next() {
		//exhaustruct:ignore
		var deployment = QueuedDeployment{}
		err = rows.Scan(&deployment.Created, &deployment.Env, &deployment.App, &deployment.ReleaseNumbers.Version, &deployment.ReleaseNumbers.Revision)
		if err != nil {
			return nil, fmt.Errorf("error scanning deployment attempts row from DB. Error: %w", err)
		}
		result = append(result, &deployment)
	}
	return result, nil
}

func (h *DBHandler) dbWriteDeploymentAttemptInternal(ctx context.Context, tx *sql.Tx, deployment *QueuedDeployment) error {
	span, ctx := tracer.StartSpanFromContext(ctx, "dbWriteDeploymentAttemptInternal")
	defer span.Finish()

	nullVersion := NewNullUInt(deployment.ReleaseNumbers.Version)
	now, err := h.DBReadTransactionTimestamp(ctx, tx)
	if err != nil {
		return fmt.Errorf("dbWriteDeploymentAttemptInternal unable to get transaction timestamp: %w", err)
	}

	insertQuery := h.AdaptQuery(
		"INSERT INTO deployment_attempts_history (created, envName, appName, releaseVersion, revision) VALUES (?, ?, ?, ?, ?);")

	span.SetTag("insert", insertQuery)
	_, err = tx.Exec(
		insertQuery,
		*now,
		deployment.Env,
		deployment.App,
		nullVersion,
		deployment.ReleaseNumbers.Revision,
	)
	if err != nil {
		return fmt.Errorf("could not write deployment attempts table in DB. Error: %w", err)
	}

	if nullVersion.Valid {
		upsertQuery := h.AdaptQuery(
			`
INSERT INTO deployment_attempts_latest (
	created,
	envName,
	appName,
	releaseVersion,
	revision
) VALUES (?, ?, ?, ?, ?)
ON CONFLICT (appName, envName) DO UPDATE SET
	created = excluded.created,
	releaseVersion = excluded.releaseVersion,
	revision = excluded.revision;
		`)

		span.SetTag("upsert", upsertQuery)
		_, err = tx.Exec(
			upsertQuery,
			*now,
			deployment.Env,
			deployment.App,
			nullVersion,
			deployment.ReleaseNumbers.Revision,
		)
		if err != nil {
			return fmt.Errorf("could not write deployment_attempts_latest table in DB. Error: %w", err)
		}
	} else {
		deleteQuery := h.AdaptQuery(
			`
DELETE FROM deployment_attempts_latest WHERE
	appName = ?
	AND
	envName = ?
			`)
		span.SetTag("delete", deleteQuery)
		_, err = tx.Exec(
			deleteQuery,
			deployment.App,
			deployment.Env,
		)
		if err != nil {
			return fmt.Errorf("could not delete from deployment_attempts_latest table in DB. Error: %w", err)
		}
	}

	return nil
}

func (h *DBHandler) MapEnvNamesToDeployment(ctx context.Context, transaction *sql.Tx, id TransformerID) (map[types.EnvName]Deployment, error) {
	deployments, err := h.DBSelectDeploymentsByTransformerID(ctx, transaction, id)
	if err != nil {
		return nil, err
	}
	deploymentsMap := make(map[types.EnvName]Deployment)

	for _, currentDeployment := range deployments {
		deploymentsMap[currentDeployment.Env] = currentDeployment
	}
	return deploymentsMap, nil
}

func NewNullUInt(s *uint64) sql.NullInt64 {
	if s == nil {
		return sql.NullInt64{
			Int64: 0,
			Valid: false,
		}
	}
	conv := int64(*s)
	return sql.NullInt64{
		Int64: conv,
		Valid: true,
	}
}
