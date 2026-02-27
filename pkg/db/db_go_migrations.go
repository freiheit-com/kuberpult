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

	"go.uber.org/zap"
	"gopkg.in/DataDog/dd-trace-go.v1/ddtrace/tracer"

	"github.com/freiheit-com/kuberpult/pkg/logger"
)

const GoMigration_AppsHistory = "AppsHistory"

func (h *DBHandler) DBInsertGoMigrationCutoff(ctx context.Context, tx *sql.Tx, migrationName string) error {
	timestamp, err := h.DBReadTransactionTimestamp(ctx, tx)
	if err != nil {
		return fmt.Errorf("read transaction timestamp from DB. Error: %w", err)
	}

	insertQuery := h.AdaptQuery(`
		INSERT INTO go_migration_cutoff (migration_done_at, migration_name)
		VALUES (?, ?);`)

	_, err = tx.ExecContext(
		ctx,
		insertQuery,
		timestamp,
		migrationName,
	)
	if err != nil {
		return fmt.Errorf("could not write to go_migration_cutoff table from DB: %w", err)
	}
	return nil
}

func (h *DBHandler) DBHasGoMigrationCutoff(ctx context.Context, tx *sql.Tx, migrationName string) (bool, error) {
	selectQuery := h.AdaptQuery(`
		SELECT migration_done_at
		FROM go_migration_cutoff
		WHERE migration_name=?
		LIMIT 1;
	`)
	rows, err := tx.QueryContext(
		ctx,
		selectQuery,
		migrationName,
	)
	if err != nil {
		return false, fmt.Errorf("could not query go_migration_cutoff table from DB: %w", err)
	}
	defer func(rows *sql.Rows) {
		err := rows.Close()
		if err != nil {
			logger.FromContext(ctx).Warn("rows could not be closed: %v", zap.Error(err))
		}
	}(rows)
	return rows.Next(), nil
}

type DBMigration struct {
	migrationName string
	migrationFunc func(ctx context.Context, tx *sql.Tx) error
}

func (h *DBHandler) RunCustomMigrations(ctx context.Context) (err error) {
	span, ctx := tracer.StartSpanFromContext(ctx, "RunCustomMigrations")
	defer func() {
		span.Finish(tracer.WithError(err))
	}()

	migrations := []DBMigration{
		{
			migrationName: GoMigration_AppsHistory,
			migrationFunc: h.DBMigrateAppsHistoryToAppsTeamsHistory,
		},
	}

	for _, migration := range migrations {
		err = h.WithTransaction(ctx, false, func(ctx context.Context, transaction *sql.Tx) error {
			// check if the migration already ran
			hasMigration, err := h.DBHasGoMigrationCutoff(ctx, transaction, migration.migrationName)
			if err != nil {
				return err
			}
			if hasMigration {
				return nil
			}

			// run the migration
			err = migration.migrationFunc(ctx, transaction)
			if err != nil {
				return err
			}

			// mark the migration as done
			return h.DBInsertGoMigrationCutoff(ctx, transaction, migration.migrationName)
		})
		if err != nil {
			return err
		}
	}
	return nil
}
