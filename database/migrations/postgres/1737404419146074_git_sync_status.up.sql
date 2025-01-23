CREATE TABLE IF NOT EXISTS git_sync_status
(
    eslVersion SERIAL,
    created TIMESTAMP,
    transformerID  INTEGER,
    envName VARCHAR,
    appName VARCHAR,
    status INTEGER,
    PRIMARY KEY(eslVersion, appName, envName)
);
