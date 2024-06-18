CREATE TABLE IF NOT EXISTS deployment_attempts
(
    eslVersion INTEGER, -- internal ID for ESL
    created TIMESTAMP,
    envName VARCHAR,
    appName VARCHAR,
    queuedReleaseVersion BIGINT NULL,
    PRIMARY KEY(eslVersion, appName, envName)
);