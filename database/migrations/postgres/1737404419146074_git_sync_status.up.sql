CREATE TABLE IF NOT EXISTS git_sync_status
(
    created TIMESTAMP,
    transformerID  INTEGER,
    envName VARCHAR,
    appName VARCHAR,
    status INTEGER,
    PRIMARY KEY(appName, envName)
);
