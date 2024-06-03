CREATE TABLE IF NOT EXISTS application_locks
(
    eslVersion INTEGER, -- internal ID for ESL
    created TIMESTAMP,
    appName VARCHAR,
    envName VARCHAR,
    metadata VARCHAR,
    PRIMARY KEY(eslVersion, appName, envName)
);
