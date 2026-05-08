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
	"errors"
	"fmt"
	"time"

	"go.uber.org/zap"
	"gopkg.in/DataDog/dd-trace-go.v1/ddtrace/tracer"

	"github.com/freiheit-com/kuberpult/pkg/logging"
)

/*
The cutoff table records the highest ESL version that has been fully processed by the
manifest-repo-export-service and successfully committed to the git manifest repository.
On startup, the service reads the maximum eslVersion here to resume replay from that point.
Only manifest-repo-export-service writes to this table; cd-service only reads it.
*/
const cutoffTable = "cutoff"

func DBReadCutoff(h *DBHandler, ctx context.Context, tx *sql.Tx) (*EslVersion, error) {
	selectQuery := h.AdaptQuery(`SELECT eslVersion
		FROM ` + cutoffTable + `
		ORDER BY eslVersion DESC
		LIMIT 1;`)
	rows, err := tx.QueryContext(
		ctx,
		selectQuery,
	)
	if err != nil {
		return nil, fmt.Errorf("could not query cutoff table from DB. Error: %w", err)
	}
	defer func(rows *sql.Rows) {
		err := rows.Close()
		if err != nil {
			logging.Error(ctx, "cutoff: row closing error", zap.Error(err))
		}
	}(rows)

	var eslVersion EslVersion
	var eslVersionPtr *EslVersion = nil
	if rows.Next() {
		err := rows.Scan(&eslVersion)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return nil, nil
			}
			return nil, fmt.Errorf("cutoff: Error scanning row from DB. Error: %w", err)
		}
		eslVersionPtr = &eslVersion
	}
	err = rows.Close()
	if err != nil {
		return nil, fmt.Errorf("row closing error: %v", err)
	}
	err = rows.Err()
	if err != nil {
		return nil, fmt.Errorf("row has error: %v", err)
	}
	return eslVersionPtr, nil
}

func DBWriteCutoff(h *DBHandler, ctx context.Context, tx *sql.Tx, eslVersion EslVersion) (err error) {
	span, ctx := tracer.StartSpanFromContext(ctx, "DBWriteCutoff")
	defer func() {
		span.Finish(tracer.WithError(err))
	}()

	insertQuery := h.AdaptQuery(`INSERT INTO ` + cutoffTable + ` (eslVersion, processedTime)
		VALUES (?, ?);`)
	span.SetTag("query", insertQuery)

	_, err = tx.ExecContext(
		ctx,
		insertQuery,
		eslVersion,
		time.Now().UTC(),
	)
	if err != nil {
		return fmt.Errorf("could not write to cutoff table from DB. Error: %w", err)
	}
	return nil
}
