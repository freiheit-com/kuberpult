CREATE TABLE IF NOT EXISTS all_deployments
(
    eslVersion INTEGER NOT NULL, -- internal ID for ESL
    created TIMESTAMP NOT NULL,
    appName VARCHAR NOT NULL,
    json VARCHAR NOT NULL, -- Stores map from environment to (deployed) release number
    PRIMARY KEY(eslVersion, appName)
);
