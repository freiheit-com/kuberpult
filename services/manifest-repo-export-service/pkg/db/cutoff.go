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

package cutoff

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"github.com/freiheit-com/kuberpult/pkg/db"
	"github.com/freiheit-com/kuberpult/pkg/logger"
	"gopkg.in/DataDog/dd-trace-go.v1/ddtrace/tracer"
	"time"
)

func DBReadCutoff(h *db.DBHandler, ctx context.Context, tx *sql.Tx) (*db.EslId, error) {
	span, _ := tracer.StartSpanFromContext(ctx, "DBReadCutoff")
	defer span.Finish()

	selectQuery := h.AdaptQuery("SELECT eslId FROM cutoff ORDER BY eslId DESC LIMIT 1;")
	span.SetTag("query", selectQuery)
	rows, err := tx.QueryContext(
		ctx,
		selectQuery,
	)
	if err != nil {
		return nil, fmt.Errorf("could not query cutoff table from DB. Error: %w\n", err)
	}
	defer func(rows *sql.Rows) {
		err := rows.Close()
		if err != nil {
			logger.FromContext(ctx).Sugar().Warnf("cutoff: row closing error: %v", err)
		}
	}(rows)

	var eslId db.EslId
	var eslIdPtr *db.EslId = nil
	if rows.Next() {
		err := rows.Scan(&eslId)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return nil, nil
			}
			return nil, fmt.Errorf("cutoff: Error scanning row from DB. Error: %w\n", err)
		}
		eslIdPtr = &eslId
	}

	err = db.CloseRows(rows)
	if err != nil {
		return nil, fmt.Errorf("row closing error: %v\n", err)
	}
	return eslIdPtr, nil
}

func DBWriteCutoff(h *db.DBHandler, ctx context.Context, tx *sql.Tx, eslId db.EslId) error {
	span, _ := tracer.StartSpanFromContext(ctx, "DBWriteCutoff")
	defer span.Finish()

	insertQuery := h.AdaptQuery("INSERT INTO cutoff (eslId, processedTime) VALUES (?, ?);")
	span.SetTag("query", insertQuery)

	_, err := tx.ExecContext(
		ctx,
		insertQuery,
		eslId,
		time.Now().UTC(),
	)
	if err != nil {
		return fmt.Errorf("could not write to cutoff table from DB: %w", err)
	}
	return nil
}
