-- Requires event_sourcing_light table to be created
CREATE TABLE IF NOT EXISTS events
(
    uuid VARCHAR(64) primary key,
    timestamp TIMESTAMP,
    commitHash VARCHAR(64),
    eventType VARCHAR(32),
    json VARCHAR
);

CREATE INDEX IF NOT EXISTS commitHashIdx on events (commitHash);