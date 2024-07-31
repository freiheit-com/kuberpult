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
	"time"

	"github.com/freiheit-com/kuberpult/pkg/db"
	"github.com/freiheit-com/kuberpult/pkg/logger"
)

const QueuedDeploymentsTable = "queued_deployments"

type QueuedDeployment struct {
	Id       int64
	Manifest []byte
}

func WriteQueuedDeployment(ctx context.Context, manifest []byte, dbHandler *db.DBHandler) error {
	err := dbHandler.WithTransaction(ctx, false, func(ctx context.Context, transaction *sql.Tx) error {
		insertQuery := dbHandler.AdaptQuery(fmt.Sprintf("INSERT INTO %s (created_at, manifest, processed) VALUES (?, ?, ?);", QueuedDeploymentsTable))
		_, err := transaction.Exec(insertQuery, time.Now().UTC(), manifest, false)
		if err != nil {
			return err
		}
		return nil
	})
	return err
}

func GetQueuedDeployments(ctx context.Context, dbHandler *db.DBHandler) ([]*QueuedDeployment, error) {
	queuedDeployments := []*QueuedDeployment{}
	err := dbHandler.WithTransaction(ctx, true, func(ctx context.Context, transaction *sql.Tx) error {
		selectQuery := dbHandler.AdaptQuery(fmt.Sprintf("SELECT id, manifest FROM %s WHERE processed is false ORDER BY id ASC", QueuedDeploymentsTable))
		rows, err := transaction.QueryContext(ctx, selectQuery)
		if err != nil {
			return fmt.Errorf("could not query %s table: Error: %w", QueuedDeploymentsTable, err)
		}
		defer func(rows *sql.Rows) {
			err := rows.Close()
			if err != nil {
				logger.FromContext(ctx).Sugar().Warnf("%s: row closing error: %v", QueuedDeploymentsTable, err)
			}
		}(rows)
		for rows.Next() {
			var id int64
			var manifest []byte
			err := rows.Scan(&id, &manifest)
			if err != nil {
				// If an error occurred here, we skip and will retry in the next processing call.
				logger.FromContext(ctx).Sugar().Warnf("failed to scan row: %v", err)
				continue
			}
			queuedDeployments = append(queuedDeployments, &QueuedDeployment{
				Id:       id,
				Manifest: manifest,
			})
		}
		if err = rows.Err(); err != nil {
			return fmt.Errorf("error iterating over rows: %v", err)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return queuedDeployments, nil
}
