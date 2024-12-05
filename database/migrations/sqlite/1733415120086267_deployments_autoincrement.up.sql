CREATE TABLE IF NOT EXISTS deployments_new
(
    created TIMESTAMP,
    releaseVersion BIGINT NULL,
    appName VARCHAR,
    envName VARCHAR,
    metadata VARCHAR,
    transformereslVersion INTEGER DEFAULT 0,
    version INTEGER PRIMARY KEY AUTOINCREMENT,
    FOREIGN KEY(transformereslVersion) REFERENCES event_sourcing_light(eslVersion)
);

INSERT INTO deployments_new (created, releaseversion, appname, envname, metadata, transformereslversion)
SELECT created, releaseversion, appname, envname, metadata, transformereslversion
FROM deployments
ORDER BY eslversion;

DROP TABLE IF EXISTS deployments;
ALTER TABLE deployments_new RENAME TO deployments;

CREATE INDEX IF NOT EXISTS deployments_idx ON deployments (appName, envname);