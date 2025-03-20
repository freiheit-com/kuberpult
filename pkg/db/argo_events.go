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
	"github.com/freiheit-com/kuberpult/pkg/logger"
	"github.com/freiheit-com/kuberpult/pkg/tracing"
	"time"
)

type ArgoEvent struct {
	App       string
	Env       string
	JsonEvent []byte
	Discarded bool
}

func (h *DBHandler) InsertArgoEvents(ctx context.Context, tx *sql.Tx, events []*ArgoEvent) error {
	span, ctx, onErr := tracing.StartSpanFromContext(ctx, "InsertArgoEvents")
	defer span.Finish()
	if h == nil {
		return nil
	}
	if tx == nil {
		return onErr(fmt.Errorf("InsertArgoEvents: no transaction provided"))
	}
	queryTemplate := `INSERT INTO argo_cd_events (created, app, env, json, discarded)
	VALUES ('%s', '%s', '%s', '%s', %t)
	ON CONFLICT(app, env)
	DO UPDATE SET created = excluded.created, json = excluded.json, discarded = excluded.discarded;`
	now, err := h.DBReadTransactionTimestamp(ctx, tx)

	if err != nil {
		return fmt.Errorf("could not insert argo events into database: %v", err)
	}
	currentQuery := ""

	for _, currentEvent := range events {
		currentQuery += fmt.Sprintf(queryTemplate, now.Format(time.RFC3339), currentEvent.App, currentEvent.Env, currentEvent.JsonEvent, currentEvent.Discarded)
	}
	_, err = tx.ExecContext(ctx, currentQuery)
	return err
}

func (h *DBHandler) DBReadArgoEvent(ctx context.Context, tx *sql.Tx, appName, envName string) (*ArgoEvent, error) {
	span, ctx, onErr := tracing.StartSpanFromContext(ctx, "DBReadArgoEvent")
	defer span.Finish()
	if h == nil {
		return nil, nil
	}
	if tx == nil {
		return nil, onErr(fmt.Errorf("DBReadArgoEvent: no transaction provided"))
	}

	selectQuery := h.AdaptQuery("SELECT app, env, json, discarded FROM argo_cd_events WHERE app = ? AND env = ? LIMIT 1;")
	row, err := tx.QueryContext(
		ctx,
		selectQuery,
		appName,
		envName,
	)
	if err != nil {
		return nil, onErr(fmt.Errorf("error reading argo cd events . Error: %w\n", err))
	}
	defer func(rows *sql.Rows) {
		err := rows.Close()
		if err != nil {
			logger.FromContext(ctx).Sugar().Warnf("row closing error: %v", err)
		}
	}(row)

	event := ArgoEvent{}
	if row.Next() {
		err := row.Scan(&event.App, &event.Env, &event.JsonEvent, &event.Discarded)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return nil, nil
			}
			return nil, onErr(fmt.Errorf("Error table for next argo_cd_events. Error: %w\n", err))
		}
	}
	err = closeRows(row)
	if err != nil {
		return nil, onErr(err)
	}
	return &event, nil
}
