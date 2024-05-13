CREATE TABLE IF NOT EXISTS events
(
    uuid VARCHAR(64),
    timestamp TIMESTAMP,
    commitHash VARCHAR(64),
    eventType VARCHAR(32),
    json VARCHAR(1024),
    PRIMARY KEY(uuid)
);

CREATE INDEX commitHashIdx on events (commitHash);