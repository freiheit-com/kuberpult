CREATE TABLE IF NOT EXISTS releases_new
(
    eslVersion INTEGER, -- internal ID for ESL
    releaseVersion INTEGER, -- release ID given by the client that calls CreateApplicationVersion
    created TIMESTAMP,
    appName VARCHAR,
    manifests VARCHAR, -- json blob, map where each key is an environment and each value is a manifest
    metadata VARCHAR,  -- json blob about the release, sourceAuthor, sourceCommitId, etc
    deleted  BOOLEAN,
    environments VARCHAR,
    PRIMARY KEY(eslVersion, releaseVersion, appName)
);

INSERT INTO releases_new(eslVersion, releaseVersion, created, appName, manifests, metadata, deleted)
SELECT eslVersion, releaseVersion, created, appName, manifests, metadata, deleted
FROM releases;

DROP TABLE IF EXISTS releases;
ALTER TABLE releases_new RENAME TO releases;
