CREATE TABLE IF NOT EXISTS queued_deployments
(
    id INTEGER PRIMARY KEY autoincrement,
    created_at TIMESTAMP NOT NULL,
    manifest VARCHAR NOT NULL,
    processed BOOLEAN NOT NULL,
    processed_at TIMESTAMP
);
