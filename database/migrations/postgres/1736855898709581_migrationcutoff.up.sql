-- Active: 1723128250488@@127.0.0.1@5432@kuberpult
CREATE TABLE IF NOT EXISTS custom_migration_cutoff
(
    migration_done_at TIMESTAMP NOT NULL,
    kuberpult_version varchar(100) PRIMARY KEY -- the version as it appears on GitHub, e.g. "1.2.3"
);
