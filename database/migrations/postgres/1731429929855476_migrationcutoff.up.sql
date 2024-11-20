CREATE TABLE IF NOT EXISTS custom_migration_cutoff
(
    migrationDoneAt TIMESTAMP NOT NULL,
    kuberpultVersion varchar(100) PRIMARY KEY -- the version as it appears on GitHub, e.g. "1.2.3"
);
