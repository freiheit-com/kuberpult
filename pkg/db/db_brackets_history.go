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
	"gopkg.in/DataDog/dd-trace-go.v1/ddtrace/tracer"

	"github.com/freiheit-com/kuberpult/pkg/logger"
)

// BracketRow contains all data for one row in the table brackets_history
type BracketRow struct {
	CreatedAt           time.Time
	AllBracketsJsonBlob BracketJsonBlob
}

type AppNames = []types.AppName

type BracketJsonBlob struct {
	BracketMap map[types.ArgoBracketName]AppNames
}

func toJson(data BracketJsonBlob) ([]byte, error) {
	return json.Marshal(data)
}

func fromJson(data []byte) (BracketJsonBlob, error) {
	result := BracketJsonBlob{}
	err := json.Unmarshal(data, &result)
	return result, err
}

func DBSelectBracketHistoryByTimestamp(h *DBHandler, ctx context.Context, tx *sql.Tx, timestamp time.Time) (result *BracketRow, err error) {
	span, ctx := tracer.StartSpanFromContext(ctx, "DBSelectBracketHistoryByTimestamp")
	defer func() {
		span.Finish(tracer.WithError(err))
	}()

	selectQuery := h.AdaptQuery(`
		SELECT created, all_brackets
		FROM brackets_history
		WHERE created_at <= (?)  -- get the rows that existed at the given time
		ORDER BY created_at DESC -- but only get the newest row
		LIMIT 1
	;`)
	rows, err := tx.QueryContext(
		ctx,
		selectQuery,
		timestamp,
	)
	if err != nil {
		return nil, fmt.Errorf("could not query cutoff table from DB. Error: %w", err)
	}
	defer func(rows *sql.Rows) {
		err := rows.Close()
		if err != nil {
			logger.FromContext(ctx).Sugar().Warnf("cutoff: row closing error: %v", err)
		}
	}(rows)
	if rows.Next() {
		result, err = processBracketHistoryRow(rows)
		if err != nil {
			err2 := closeRows(rows)
			return nil, errors.Join(err, err2)
		}
	}
	err = closeRows(rows)
	if err != nil {
		return nil, err
	}
	return result, nil
}

func processBracketHistoryRow(rows *sql.Rows) (*BracketRow, error) {
	var rawJson []byte
	result := BracketRow{}
	err := rows.Scan(&result.CreatedAt, &rawJson)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("scanning row from DB: %w", err)
	}
	parsedJson, err := fromJson(rawJson)
	if err != nil {
		return nil, fmt.Errorf("row contains invalid json: <%s>: %w", rawJson, err)
	}
	result.AllBracketsJsonBlob = parsedJson
	return &result, nil
}

func DBInsertBracketHistory(h *DBHandler, ctx context.Context, tx *sql.Tx, bracketRow BracketRow) (err error) {
	span, ctx := tracer.StartSpanFromContext(ctx, "DBInsertBracketHistory")
	defer func() {
		span.Finish(tracer.WithError(err))
	}()

	insertQuery := h.AdaptQuery(`
		INSERT INTO ${brackets_history} (eslVersion, processedTime)
		VALUES (?, ?)
	;`)

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
