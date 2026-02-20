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
	"slices"
	"time"

	"gopkg.in/DataDog/dd-trace-go.v1/ddtrace/tracer"

	"github.com/freiheit-com/kuberpult/pkg/types"
)

const bracketsHistoryTable = "brackets_history"

// BracketRow represents one row in the table brackets_history
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

func HandleBracketsUpdate(h *DBHandler, ctx context.Context, tx *sql.Tx, app types.AppName, newBracketName types.ArgoBracketName, now time.Time) error {
	//now, err := h.DBReadTransactionTimestamp(ctx, tx)
	//if err != nil {
	//	return fmt.Errorf("HandleBracketsUpdate could not get timestamp: %w", err)
	//}

	bracketRow, err := DBSelectBracketHistoryByTimestamp(h, ctx, tx, &now)
	if err != nil {
		return fmt.Errorf("HandleBracketsUpdate could not get newBracketName by timestamp: %w", err)
	}
	if bracketRow == nil {
		bracketRow = &BracketRow{
			CreatedAt: now,
			AllBracketsJsonBlob: BracketJsonBlob{
				BracketMap: make(map[types.ArgoBracketName]AppNames),
			},
		}
	}

	// find the old bracketName of the app:
	for oldBracketName, appNames := range bracketRow.AllBracketsJsonBlob.BracketMap {
		oldIndex := slices.Index(appNames, app)
		if oldIndex >= 0 { // found the app
			if newBracketName == oldBracketName {
				// same bracket, nothing to do
				return nil
			}
			// we found the app but in a different bracket
			// 1) remove app from old bracket:
			appNames = slices.Delete(appNames, oldIndex, oldIndex)
			bracketRow.AllBracketsJsonBlob.BracketMap[oldBracketName] = appNames
			bracketRow.CreatedAt = now
		}
	}

	newBracketApps, ok := bracketRow.AllBracketsJsonBlob.BracketMap[newBracketName]
	if ok {
		// bracket exists, just add the app
		bracketRow.AllBracketsJsonBlob.BracketMap[newBracketName] = append(newBracketApps, app)
	} else {
		// bracket is new, just add it with only our app:
		bracketRow.AllBracketsJsonBlob.BracketMap[newBracketName] = AppNames{app}
	}

	err = DBInsertBracketHistory(h, ctx, tx, *bracketRow)
	if err != nil {
		return fmt.Errorf("HandleBracketsDeletion could not get insert new bracket: %w", err)
	}

	return nil
}

func HandleBracketsDeletion(h *DBHandler, ctx context.Context, tx *sql.Tx, app types.AppName, deletionBracketName types.ArgoBracketName, now time.Time) error {
	bracketRow, err := DBSelectBracketHistoryByTimestamp(h, ctx, tx, &now)
	if err != nil {
		return fmt.Errorf("HandleBracketsDeletion could not get newBracketName by timestamp: %w", err)
	}
	if bracketRow == nil {
		// bracket did not exist, that's odd, but not an error
		return nil
	}

	// find the old bracketName of the app and remove it:
	for oldBracketName, appNames := range bracketRow.AllBracketsJsonBlob.BracketMap {
		oldIndex := slices.Index(appNames, app)
		if oldIndex >= 0 { // found the app
			if deletionBracketName == oldBracketName {
				bracketRow.CreatedAt = now
				// we found the app, now remove it:
				appNames = slices.Delete(appNames, oldIndex, oldIndex+1)
				if len(appNames) == 0 {
					// last app in the bracket, delete the bracket:
					delete(bracketRow.AllBracketsJsonBlob.BracketMap, oldBracketName)
				} else {
					// there are other apps, keep them:
					bracketRow.AllBracketsJsonBlob.BracketMap[oldBracketName] = appNames
				}
				break
			}
		}
	}

	err = DBInsertBracketHistory(h, ctx, tx, *bracketRow)
	if err != nil {
		return fmt.Errorf("HandleBracketsDeletion could not get insert new bracket: %w", err)
	}
	return nil
}

func DBSelectBracketHistoryByTimestamp(h *DBHandler, ctx context.Context, tx *sql.Tx, optionalTimestamp *time.Time) (result *BracketRow, err error) {
	span, ctx := tracer.StartSpanFromContext(ctx, "DBSelectBracketHistoryByTimestamp")
	defer func() {
		span.Finish(tracer.WithError(err))
	}()

	whereQuery := ""
	args := []any{}
	if optionalTimestamp != nil {
		whereQuery = `WHERE created_at <= (?) -- get the rows that existed at the given time`
		args = append(args, optionalTimestamp)
	}
	selectQuery := h.AdaptQuery(`
		SELECT created_at, all_brackets
		FROM ` + bracketsHistoryTable + `
		` + whereQuery + `
		ORDER BY created_at DESC -- but only get the newest row
		LIMIT 1
	;`)
	rows, err := tx.QueryContext(
		ctx,
		selectQuery,
		args...,
	)
	if err != nil {
		return nil, fmt.Errorf("could not query cutoff table from DB. Error: %w", err)
	}
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
		INSERT INTO ` + bracketsHistoryTable + ` (created_at, all_brackets)
		VALUES (?, ?)
	;`)

	jsonBytes, err := toJson(bracketRow.AllBracketsJsonBlob)
	_, err = tx.ExecContext(
		ctx,
		insertQuery,
		bracketRow.CreatedAt,
		jsonBytes,
	)
	if err != nil {
		return fmt.Errorf("failed to insert into '%s'. Error: %w", bracketsHistoryTable, err)
	}
	return nil
}
