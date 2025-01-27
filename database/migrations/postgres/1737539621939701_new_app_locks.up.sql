-- Active: 1731876050071@@127.0.0.1@5432@postgres
SET TRANSACTION ISOLATION LEVEL SERIALIZABLE;
-- create app_locks table if it doesn't exist
CREATE TABLE IF NOT EXISTS app_locks
(
    created TIMESTAMP,
    lockid VARCHAR,
    envname VARCHAR,
    appname VARCHAR,
    metadata VARCHAR,
    PRIMARY KEY(appname, envname, lockid)
);
