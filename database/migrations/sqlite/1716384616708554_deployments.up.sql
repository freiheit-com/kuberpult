CREATE TABLE IF NOT EXISTS deployments
(
    eslVersion INTEGER, -- internal ID for ESL
    created TIMESTAMP,
    releaseVersion BIGINT NULLABLE, -- this ID is provided by the API caller
    appName VARCHAR,
    envName VARCHAR,
    metadata VARCHAR,
    PRIMARY KEY(eslVersion, appName, envName)
);
