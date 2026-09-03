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

package db_history

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/lib/pq"
	"gopkg.in/DataDog/dd-trace-go.v1/ddtrace/tracer"

	"github.com/freiheit-com/kuberpult/pkg/types"
)

type DeploymentShort struct {
	ReleaseVersion types.ReleaseVersion
	Revision       types.Revision
}
type DeploymentMap map[types.AppName]DeploymentShort

// DBSelectAppsWithDeploymentInEnvAtTimestamp returns all apps that had a deployment in the given env at the given timestamp:
func DBSelectAppsWithDeploymentInEnvAtTimestamp(ctx context.Context, tx *sql.Tx, envSelector types.EnvName, ts time.Time, appNames []types.AppName) (_ DeploymentMap, err error) {
	span, ctx := tracer.StartSpanFromContext(ctx, "DBSelectAppsWithDeploymentInEnvAtTimestamp")
	defer func() {
		span.Finish(tracer.WithError(err))
	}()
	span.SetTag("kuberpultEnvironment", envSelector)
	span.SetTag("numApps", len(appNames))
	selectQuery := `
		 SELECT
			  a.appname,
			  d.releaseversion,
			  d.revision
		  FROM unnest($1::text[]) AS a(appname)  -- "unnest" simply converts our slice into a row
		--  For manually using this query in psql, replace this line with:
		--  FROM unnest((SELECT array_agg(appname::text) FROM apps)) AS a(appname)

		--- Then we join lateral ("subquery") in order to get the latest deployment.
	    --- This will skip apps that have now deployment - that's ok, we cannot deploy those anyway!
		  JOIN LATERAL (
			  SELECT
				  releaseversion,
				  revision
			  FROM deployments_history
			  WHERE appname = a.appname
				  AND envname = $2
				  AND created <= $3
			  -- Note that we cannot filter "releaseVersion IS NULL" here, even though we later filter out null deployments,
			  -- because that would not give us the LATEST deployment.
			  ORDER BY created DESC, version DESC
			  LIMIT 1
		  ) AS d ON TRUE
		  ORDER BY a.appname;`
	rows, err := tx.QueryContext(
		ctx,
		selectQuery,
		pq.Array(appNames),
		envSelector,
		ts,
	)
	if err != nil {
		return nil, fmt.Errorf("could not select historic deployment on env %s from DB: %w", envSelector, err)
	}
	defer func() {
		_ = rows.Close()
	}()
	result := make(DeploymentMap)
	for rows.Next() {
		appName, deployment, err := processOneDeploymentsForEnv(rows)
		if err != nil {
			return nil, err
		}
		result[appName] = deployment
	}
	span.SetTag("resultLen", len(result))
	err = rows.Err()
	if err != nil {
		return nil, fmt.Errorf("could not select historic deployment on env %s from DB: %w", envSelector, err)
	}
	return result, nil
}

func processOneDeploymentsForEnv(rows *sql.Rows) (types.AppName, DeploymentShort, error) {
	var c = DeploymentShort{
		ReleaseVersion: nil,
		Revision:       0,
	}
	var sqlReleaseVersion sql.NullInt64
	var app types.AppName
	err := rows.Scan(&app, &sqlReleaseVersion, &c.Revision)
	if err != nil {
		return app, c, fmt.Errorf("error scanning deployments row from DB. Error: %w", err)
	}
	if sqlReleaseVersion.Valid {
		conv := uint64(sqlReleaseVersion.Int64)
		c.ReleaseVersion = &conv
	}
	return app, c, nil
}
