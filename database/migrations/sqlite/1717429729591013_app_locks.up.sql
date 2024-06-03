CREATE TABLE IF NOT EXISTS application_locks
(
    eslVersion INTEGER, -- internal ID for ESL
    created TIMESTAMP,
    envName VARCHAR,
    appName VARCHAR,
    metadata VARCHAR,
    PRIMARY KEY(eslVersion, appName, envName)
);
