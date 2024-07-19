CREATE TABLE IF NOT EXISTS cutoff_new
(
    processedTime TIMESTAMP,
    eslVersion INTEGER,
    FOREIGN KEY (eslversion) REFERENCES event_sourcing_light(eslId)
);
INSERT INTO cutoff_new (processedTime, eslVersion)
SELECT processedTime, eslId
FROM cutoff;
DROP TABLE IF EXISTS cutoff;
ALTER TABLE cutoff_new RENAME TO cutoff;

CREATE TABLE IF NOT EXISTS event_sourcing_light_new 
(
    eslVersion INTEGER PRIMARY KEY autoincrement,
    created TIMESTAMP,
    event_type VARCHAR(255),
    json VARCHAR
);
INSERT INTO event_sourcing_light_new (eslVersion, created, event_type, json)
SELECT eslId, created, event_type, json
FROM event_sourcing_light;
DROP TABLE IF EXISTS event_sourcing_light;
ALTER TABLE event_sourcing_light_new RENAME TO event_sourcing_light;

CREATE TABLE IF NOT EXISTS event_sourcing_light_failed_new
(
    eslVersion INTEGER PRIMARY KEY autoincrement,
    created TIMESTAMP NOT NULL,
    event_type VARCHAR(255) NOT NULL,
    json VARCHAR NOT NULL
);
INSERT INTO event_sourcing_light_failed_new (eslVersion, created, event_type, json)
SELECT eslId, created, event_type, json
FROM event_sourcing_light_failed;
DROP TABLE IF EXISTS event_sourcing_light_failed;
ALTER TABLE event_sourcing_light_failed_new RENAME TO event_sourcing_light_failed;

CREATE TABLE IF NOT EXISTS overview_cache_new
(
    eslVersion INTEGER PRIMARY KEY,
    timestamp TIMESTAMP,
    json VARCHAR
);
INSERT INTO overview_cache_new(eslId, timestamp, json)
SELECT eslVersion, timestamp, json
FROM overview_cache;
DROP TABLE IF EXISTS overview_cache;
ALTER TABLE overview_cache_new RENAME TO overview_cache;

CREATE TABLE IF NOT EXISTS commit_events_new
(
    uuid VARCHAR(64),
    timestamp TIMESTAMP,
    commitHash VARCHAR(64),
    eventType VARCHAR(32),
    json VARCHAR,
    transformereslVersion INTEGER,
    FOREIGN KEY (transformereslVersion) REFERENCES event_sourcing_light(eslVersion) DEFAULT 0,
    PRIMARY KEY(uuid)
);
CREATE INDEX IF NOT EXISTS commitHashIdx on commit_events_new (commitHash);
INSERT INTO commit_events_new(uuid, timestamp, commitHash, eventType, json, transformereslVersion)
SELECT uuid, timestamp, commitHash, eventType, json, transformereslId
FROM commit_events;
DROP TABLE IF EXISTS commit_events;
ALTER TABLE commit_events_new RENAME TO commit_events;

CREATE TABLE IF NOT EXISTS deployments_new
(
    eslVersion INTEGER,
    created TIMESTAMP,
    releaseVersion BIGINT NULL,
    appName VARCHAR,
    envName VARCHAR,
    metadata VARCHAR,
    transformereslVersion INTEGER,
    FOREIGN KEY(transformereslVersion) REFERENCES event_sourcing_light(eslVersion) DEFAULT 0,
    PRIMARY KEY(eslVersion, appName, envName)
);
INSERT INTO deployments_new(eslVersion, created, releaseVersion, appName, envName, metadata, transformereslVersion)
SELECT eslVersion, created, releaseVersion, appName, envName, metadata, transformereslId
FROM deployments;
DROP TABLE IF EXISTS deployments;
ALTER TABLE deployments_new RENAME TO deployments;
