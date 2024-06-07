CREATE TABLE IF NOT EXISTS application_locks
(
    eslVersion INTEGER, -- internal ID for ESL
    created TIMESTAMP,
    lockID  VARCHAR,
    envName VARCHAR,
    appName VARCHAR,
    metadata VARCHAR,
    deleted boolean,
    PRIMARY KEY(eslVersion, appName, envName, lockID)
);