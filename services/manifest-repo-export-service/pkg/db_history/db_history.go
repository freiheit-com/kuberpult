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
	"encoding/json"
	"errors"
	"fmt"
	"github.com/freiheit-com/kuberpult/pkg/db"
	"time"

	"github.com/freiheit-com/kuberpult/pkg/types"

	"github.com/freiheit-com/kuberpult/pkg/logger"
)

type DeploymentMap map[types.AppName]db.Deployment

// DBSelectAppsWithDeploymentInEnvAtTimestamp returns all apps that had a deployment in the given env at the given timestamp:
func DBSelectAppsWithDeploymentInEnvAtTimestamp(ctx context.Context, h *db.DBHandler, tx *sql.Tx, envSelector types.EnvName, ts time.Time) (DeploymentMap, error) {
	selectQuery := h.AdaptQuery(`
		SELECT
			created,
			appName,
			releaseVersion,
			envName,
			metadata,
			transformereslVersion,
			revision
		FROM (
			SELECT
				*,
				ROW_NUMBER() OVER(PARTITION BY appName, envName ORDER BY created DESC) as row_number
			FROM
				deployments_history
			WHERE	
				created <= ?
			AND
				envName = ?
		) AS subquery
		WHERE
			row_number = 1
		ORDER BY
			appName ASC, envName ASC
 	`)
	rows, err := tx.QueryContext(
		ctx,
		selectQuery,
		ts,
		envSelector,
	)
	if err != nil {
		return nil, fmt.Errorf("could not select historic deployment on env %s from DB: %w", envSelector, err)
	}
	defer func(rows *sql.Rows) {
		err := rows.Close()
		if err != nil {
			logger.FromContext(ctx).Sugar().Warnf("deployments: row closing error: %v", err)
		}
	}(rows)
	return processAllLatestDeploymentsForEnv(ctx, rows)
}

func processAllLatestDeploymentsForEnv(_ context.Context, rows *sql.Rows) (DeploymentMap, error) {
	result := make(map[types.AppName]db.Deployment)
	for rows.Next() {
		var curr = db.Deployment{
			Created: time.Time{},
			Env:     "",
			App:     "",
			ReleaseNumbers: types.ReleaseNumbers{
				Revision: 0,
				Version:  nil,
			},
			Metadata: db.DeploymentMetadata{
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
		result[types.AppName(curr.App)] = curr
	}
	return result, nil
}
