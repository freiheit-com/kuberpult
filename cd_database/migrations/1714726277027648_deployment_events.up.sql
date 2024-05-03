CREATE TABLE IF NOT EXISTS events
(
    created TIMESTAMP,
    commitHash VARCHAR(64),
    eventType VARCHAR(32),
    json VARCHAR(255),
    PRIMARY KEY(created)
);
