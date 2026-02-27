CREATE TABLE IF NOT EXISTS go_migration_cutoff (
    migration_done_at TIMESTAMP NOT NULL,
    migration_name varchar(100) PRIMARY KEY
);