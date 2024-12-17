CREATE TABLE IF NOT EXISTS deployments_history
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

INSERT INTO deployments_history (created, releaseversion, appname, envname, metadata, transformereslversion)
SELECT created, releaseversion, appname, envname, metadata, transformereslversion
FROM deployments
ORDER BY version;
DROP TABLE IF EXISTS deployments;
DROP TABLE IF EXISTS all_deployments;
CREATE TABLE IF NOT EXISTS deployments
(
    created TIMESTAMP,
    releaseVersion BIGINT NULL,
    appName VARCHAR,
    envName VARCHAR,
    metadata VARCHAR,
    transformereslVersion INTEGER DEFAULT 0,
    PRIMARY KEY (appname, envname)
    FOREIGN KEY(transformereslVersion) REFERENCES event_sourcing_light(eslVersion)
);
