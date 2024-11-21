CREATE TABLE IF NOT EXISTS releases_new
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

INSERT INTO releases_new (releaseversion, created, appname, manifests, metadata, deleted)
SELECT releaseversion, created, appname, manifests, metadata, deleted
FROM releases
ORDER BY eslversion;

DROP TABLE IF EXISTS releases;
ALTER TABLE releases_new RENAME TO releases;