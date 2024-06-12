CREATE TABLE IF NOT EXISTS deployment_attempts
(
    eslVersion INTEGER, -- internal ID for ESL
    created TIMESTAMP,
    envName VARCHAR,
    appName VARCHAR,
    queuedVersion BIGINT NULL, -- this ID is provided by the API caller
    PRIMARY KEY(eslVersion, appName, envName)
);