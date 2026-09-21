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
	"fmt"
	"slices"
	"time"

	"github.com/lib/pq"
	"gopkg.in/DataDog/dd-trace-go.v1/ddtrace/tracer"

	"github.com/freiheit-com/kuberpult/pkg/types"
)

type DeploymentRelaseInfo struct {
	ReleaseVersion *types.ReleaseVersion
	Revision       types.Revision
}
type DeploymentMap map[types.AppName]DeploymentRelaseInfo

// DBSelectAppsWithDeploymentInEnvAtTimestamp limits the given list of apps so that only apps with a non-zero deployment in the given env at the given timestamp are returned:
// For performance it is important that the list of apps is sorted.
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
	    --- This will skip apps that have no deployment - that's ok, we cannot deploy those anyway!
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

func processOneDeploymentsForEnv(rows *sql.Rows) (types.AppName, DeploymentRelaseInfo, error) {
	var c = DeploymentRelaseInfo{
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
		conv := types.ReleaseVersion(sqlReleaseVersion.Int64)
		c.ReleaseVersion = &conv
	}
	return app, c, nil
}

// GetAppsWithDeploymentAndReleaseAtTimestamp returns all apps where at the given timestamp:
//   - the latest deployment is non-null for the given environment, and
//   - the latest release is non-null for the given environment.
//
// The result represents all apps that need to be deployed at the given time.
func GetAppsWithDeploymentAndReleaseAtTimestamp(ctx context.Context, transaction *sql.Tx, dbHandler *DBHandler, parentEnvName types.EnvName, timestamp time.Time) (DeploymentMap, []AppWithTeam, error) {
	// 1) get apps with teams
	teamAppSlice, err := dbHandler.DBSelectAppsTeamsHistoryAtTimestamp(ctx, transaction, timestamp)
	if err != nil {
		return nil, nil, fmt.Errorf("could not select apps teams history: %w", err)
	}
	var appNames []types.AppName
	for _, appTeam := range teamAppSlice {
		appNames = append(appNames, appTeam.AppName)
	}

	// 2) reduce to only apps with deployment
	deploymentsPerApp, err := DBSelectAppsWithDeploymentInEnvAtTimestamp(
		ctx,
		transaction,
		parentEnvName,
		timestamp,
		appNames,
	)
	if err != nil {
		return nil, nil, fmt.Errorf("could not select apps with deployment in env at timestamp: %w", err)
	}
	// 3) reduce to only apps with deployment and release
	consideredApps := []DeployedApp{}
	for appName, releaseData := range deploymentsPerApp {
		consideredApps = append(consideredApps, DeployedApp{
			AppName:        appName,
			ReleaseVersion: releaseData.ReleaseVersion,
			Revision:       releaseData.Revision,
		})
	}
	appsWithReleaseAndDeployment, err := dbHandler.DBSelectAppTeamsWithReleaseAtTimestamp(ctx, transaction, consideredApps, parentEnvName, timestamp)
	if err != nil {
		return nil, nil, fmt.Errorf("could not select environment applications at timestamp: %w", err)
	}
	// 4) create appteam result by filtering:
	reducedTeamAppSlice := []AppWithTeam{}
	for _, teamApp := range teamAppSlice {
		if slices.Contains(appsWithReleaseAndDeployment, teamApp.AppName) {
			reducedTeamAppSlice = append(reducedTeamAppSlice, teamApp)
		}
	}
	// 5) reduce deploymentsPerApp to remove deployments without release:
	for appName := range deploymentsPerApp {
		if !slices.Contains(appsWithReleaseAndDeployment, appName) {
			delete(deploymentsPerApp, appName)
		}
	}
	return deploymentsPerApp, reducedTeamAppSlice, nil
}
