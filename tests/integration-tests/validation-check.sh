#!/bin/bash
set -eo pipefail

VALIDATION_SQL_FILE="${VALIDATION_SQL_FILE:-validation-check.sql}"

DB_PORT="${DB_PORT:-5432}"
DB_NAME="${DB_NAME:-kuberpult}"
DB_USER="${DB_USER:-postgres}"
DB_HOST="${DB_HOST:-localhost}"
DB_PASSWORD="${DB_PASSWORD:-mypassword}"

export PGPASSWORD="$DB_PASSWORD"

echo "Running queries in to check validation..."
result=$(psql -h "$DB_HOST" -U "$DB_USER" -d "$DB_NAME" -p "$DB_PORT" -f "$VALIDATION_SQL_FILE")
echo "Validation check query result:"
echo "$result"
row_counts=$(echo "$result" | grep 'rows)' | awk -F'[()]' '{print $2}' | awk '{print $1}')

echo "Row counts from all queries: $row_counts"

for count in $row_counts; do
  if (( count > 0 )); then
    echo "At least one query returned rows ($count) — failing."
    exit 1
  fi
done

unset PGPASSWORD

echo "Done."
