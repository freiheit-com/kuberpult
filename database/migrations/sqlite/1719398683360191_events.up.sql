-- Requires event_sourcing_light table to be created
CREATE TABLE IF NOT EXISTS commit_events
(
    uuid VARCHAR(64),
    timestamp TIMESTAMP,
    commitHash VARCHAR(64),
    eventType VARCHAR(32),
    json VARCHAR,
    transformerEslId INTEGER,
    PRIMARY KEY(uuid),
    FOREIGN KEY(transformerEslId) REFERENCES event_sourcing_light(eslId)
);

CREATE INDEX IF NOT EXISTS commitHashIdx on commit_events (commitHash);