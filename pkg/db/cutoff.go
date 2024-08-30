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

	"github.com/freiheit-com/kuberpult/pkg/logger"
	"gopkg.in/DataDog/dd-trace-go.v1/ddtrace/tracer"
)

func DBReadCutoff(h *DBHandler, ctx context.Context, tx *sql.Tx) (*EslVersion, error) {
	span, _ := tracer.StartSpanFromContext(ctx, "DBReadCutoff")
	defer span.Finish()

	selectQuery := h.AdaptQuery("SELECT eslVersion FROM cutoff ORDER BY eslVersion DESC LIMIT 1;")
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

	var eslVersion EslVersion
	var eslVersionPtr *EslVersion = nil
	if rows.Next() {
		err := rows.Scan(&eslVersion)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return nil, nil
			}
			return nil, fmt.Errorf("cutoff: Error scanning row from DB. Error: %w\n", err)
		}
		eslVersionPtr = &eslVersion
	}
	err = rows.Close()
	if err != nil {
		return nil, fmt.Errorf("row closing error: %v\n", err)
	}
	err = rows.Err()
	if err != nil {
		return nil, fmt.Errorf("row has error: %v\n", err)
	}
	return eslVersionPtr, nil
}

func DBWriteCutoff(h *DBHandler, ctx context.Context, tx *sql.Tx, eslVersion EslVersion) error {
	span, _ := tracer.StartSpanFromContext(ctx, "DBWriteCutoff")
	defer span.Finish()

	insertQuery := h.AdaptQuery("INSERT INTO cutoff (eslVersion, processedTime) VALUES (?, ?);")
	span.SetTag("query", insertQuery)

	_, err := tx.Exec(
		insertQuery,
		eslVersion,
		time.Now().UTC(),
	)
	if err != nil {
		return fmt.Errorf("could not write to cutoff table from DB. Error: %w\n", err)
	}
	return nil
}
