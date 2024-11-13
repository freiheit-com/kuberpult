CREATE TABLE IF NOT EXISTS migration_cutoff
(
    migrationDoneAt TIMESTAMP NOT NULL,
    kuberpultVersion SERIAL PRIMARY KEY -- the version as it appears on GitHub, e.g. "1.2.3"
);
