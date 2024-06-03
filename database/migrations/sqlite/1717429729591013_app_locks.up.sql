CREATE TABLE IF NOT EXISTS deployments
(
    eslVersion INTEGER, -- internal ID for ESL
    created TIMESTAMP,
    appName VARCHAR,
    envName VARCHAR,
    metadata VARCHAR,
    PRIMARY KEY(eslVersion, appName, envName)
);
