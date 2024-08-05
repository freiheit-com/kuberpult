CREATE TABLE IF NOT EXISTS all_deployments
(
    eslVersion INTEGER, -- internal ID for ESL
    created TIMESTAMP,
    appName VARCHAR,
    json VARCHAR, -- Stores map from environment to (deployed) release number
    PRIMARY KEY(eslVersion, appName)
);
