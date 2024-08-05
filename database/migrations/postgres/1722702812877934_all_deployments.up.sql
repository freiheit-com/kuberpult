CREATE TABLE IF NOT EXISTS all_deployments
(
    eslVersion INTEGER, -- internal ID for ESL
    created TIMESTAMP,
    appName VARCHAR,
    json VARCHAR,
    PRIMARY KEY(eslVersion, appName)
);
