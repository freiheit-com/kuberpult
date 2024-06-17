CREATE TABLE IF NOT EXISTS deployment_attempts
(
    eslVersion INTEGER, -- internal ID for ESL
    created TIMESTAMP,
    envName VARCHAR,
    appName VARCHAR,
    queuedVersion BIGINT NULL,
    PRIMARY KEY(eslVersion, appName, envName)
);