package db

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/freiheit-com/kuberpult/pkg/logger"
	"go.uber.org/zap"
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
