CREATE TABLE IF NOT EXISTS releases
(
    eslVersion INTEGER, -- internal ID for ESL
    releaseVersion INTEGER, -- release ID given by the client that calls CreateApplicationVersion
    created TIMESTAMP,
    appName VARCHAR,
    manifests VARCHAR,
    metadata VARCHAR,
    PRIMARY KEY(eslVersion, releaseVersion, appName)
);


CREATE TABLE IF NOT EXISTS all_releases
(
    eslVersion INTEGER, -- internal ID for ESL
    created TIMESTAMP,
    appName VARCHAR,
    metadata VARCHAR,
    PRIMARY KEY(eslVersion, appName)
);
