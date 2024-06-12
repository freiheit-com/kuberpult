CREATE TABLE IF NOT EXISTS releases
(
    eslVersion INTEGER, -- internal ID for ESL
    releaseVersion INTEGER, -- release ID given by the client that calls CreateApplicationVersion
    created TIMESTAMP,
    appName VARCHAR,
    envName VARCHAR,
    manifest VARCHAR,
    metadata VARCHAR,
    PRIMARY KEY(eslVersion, releaseVersion, appName, envName)
);
