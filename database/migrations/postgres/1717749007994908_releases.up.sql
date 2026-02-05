CREATE TABLE IF NOT EXISTS releases
(
    eslVersion INTEGER, -- internal ID for ESL
    releaseVersion INTEGER, -- release ID given by the client that calls CreateApplicationVersion
    created TIMESTAMP,
    appName VARCHAR,
    manifests VARCHAR, -- json blob, map where each key is an environment and each value is a manifest
    metadata VARCHAR,  -- json blob about the release, sourceAuthor, sourceCommitId, etc
    deleted  BOOLEAN,
    PRIMARY KEY(eslVersion, releaseVersion, appName)
);


-- the all_releases exists to get quick access to all releases of one app
CREATE TABLE IF NOT EXISTS all_releases
(
    eslVersion INTEGER, -- internal ID for ESL
    created TIMESTAMP,
    appName VARCHAR,
    metadata VARCHAR, -- json blob containing all releaseVersions of the given app
    PRIMARY KEY(eslVersion, appName)
);
