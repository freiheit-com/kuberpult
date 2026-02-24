package db

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/freiheit-com/kuberpult/pkg/logger"
	"go.uber.org/zap"
	"gopkg.in/DataDog/dd-trace-go.v1/ddtrace/tracer"
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

	_, err = tx.Exec(
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

func (h *DBHandler) RunCustomMigrationAppsHistory(ctx context.Context) (err error) {
	span, ctx := tracer.StartSpanFromContext(ctx, "RunCustomMigrationAppsHistory")
	defer func() {
		span.Finish(tracer.WithError(err))
	}()

	return h.WithTransaction(ctx, false, func(ctx context.Context, transaction *sql.Tx) error {
		// check if the migration already ran
		hasMigration, err := h.DBHasGoMigrationCutoff(ctx, transaction, GoMigration_AppsHistory)
		if err != nil {
			return err
		}
		if hasMigration {
			return nil
		}

		// run the migration
		err = h.DBMigrateAppsHistoryToAppsTeamsHistory(ctx, transaction)
		if err != nil {
			return err
		}

		// mark the migration as done
		return h.DBInsertGoMigrationCutoff(ctx, transaction, GoMigration_AppsHistory)
	})
}
