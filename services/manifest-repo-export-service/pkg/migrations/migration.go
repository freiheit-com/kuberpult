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

package migrations

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"github.com/freiheit-com/kuberpult/pkg/api/v1"
	"github.com/freiheit-com/kuberpult/pkg/db"
	"github.com/freiheit-com/kuberpult/pkg/logger"
	"github.com/freiheit-com/kuberpult/pkg/tracing"
)

func DBReadCustomMigrationCutoff(h *db.DBHandler, ctx context.Context, transaction *sql.Tx, requestedVersion *api.KuberpultVersion) (*api.KuberpultVersion, error) {
	span, ctx, onErr := tracing.StartSpanFromContext(ctx, "DBReadCustomMigrationCutoff")
	defer span.Finish()

	requestedVersionString := FormatKuberpultVersion(requestedVersion)

	selectQuery := h.AdaptQuery(`
SELECT kuberpult_version
FROM custom_migration_cutoff
WHERE kuberpult_version=?
LIMIT 1;`)
	span.SetTag("query", selectQuery)
	span.SetTag("requestedVersion", requestedVersionString)
	rows, err := transaction.QueryContext(
		ctx,
		selectQuery,
		requestedVersionString,
	)
	if err != nil {
		return nil, onErr(fmt.Errorf("could not query cutoff table from DB. Error: %w\n", err))
	}
	defer func(rows *sql.Rows) {
		err := rows.Close()
		if err != nil {
			logger.FromContext(ctx).Sugar().Warnf("migration_cutoff: row closing error: %v", err)
		}
	}(rows)

	if !rows.Next() {
		return nil, nil
	}
	var rawVersion string
	err = rows.Scan(&rawVersion)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, onErr(fmt.Errorf("migration_cutoff: Error scanning row from DB. Error: %w\n", err))
	}
	err = rows.Close()
	if err != nil {
		return nil, onErr(fmt.Errorf("migration_cutoff: row closing error: %v\n", err))
	}
	err = rows.Err()
	if err != nil {
		return nil, onErr(fmt.Errorf("migration_cutoff: row has error: %v\n", err))
	}

	var kuberpultVersion *api.KuberpultVersion
	kuberpultVersion, err = ParseKuberpultVersion(rawVersion)
	if err != nil {
		return nil, onErr(fmt.Errorf("migration_cutoff: Error parsing kuberpult version. Error: %w", err))
	}
	return kuberpultVersion, nil
}

func DBWriteCustomMigrationCutoff(h *db.DBHandler, ctx context.Context, tx *sql.Tx, kuberpultVersion *api.KuberpultVersion) error {
	span, ctx, onErr := tracing.StartSpanFromContext(ctx, "DBWriteCustomMigrationCutoff")
	defer span.Finish()

	timestamp, err := h.DBReadTransactionTimestamp(ctx, tx)
	if err != nil {
		return onErr(fmt.Errorf("DBWriteCustomMigrationCutoff: Error reading transaction timestamp from DB. Error: %w", err))
	}

	insertQuery := h.AdaptQuery("INSERT INTO custom_migration_cutoff (migration_done_at, kuberpult_version) VALUES (?, ?);")
	span.SetTag("query", insertQuery)

	_, err = tx.Exec(
		insertQuery,
		timestamp,
		FormatKuberpultVersion(kuberpultVersion),
	)
	if err != nil {
		return onErr(fmt.Errorf("could not write to cutoff table from DB. Error: %w\n", err))
	}
	return nil
}
