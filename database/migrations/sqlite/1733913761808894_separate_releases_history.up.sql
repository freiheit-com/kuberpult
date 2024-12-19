
CREATE TABLE IF NOT EXISTS releases_history
(
    releaseVersion INTEGER,
    created TIMESTAMP,
    appName VARCHAR,
    manifests VARCHAR,
    metadata VARCHAR,
    deleted  BOOLEAN,
    environments VARCHAR,
    version INTEGER PRIMARY KEY AUTOINCREMENT
);

INSERT INTO releases_history (releaseversion, created, appname, manifests, metadata, deleted)
SELECT releaseversion, created, appname, manifests, metadata, deleted
FROM releases
ORDER BY version;

DROP TABLE IF EXISTS releases;
DROP TABLE IF EXISTS all_releases;

CREATE TABLE IF NOT EXISTS releases
(
    releaseVersion INTEGER,
    created TIMESTAMP,
    appName VARCHAR,
    manifests VARCHAR,
    metadata VARCHAR,
    environments VARCHAR,
    PRIMARY KEY (releaseVersion, appName)
);