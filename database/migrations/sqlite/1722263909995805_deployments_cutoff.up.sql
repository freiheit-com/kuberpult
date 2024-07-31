CREATE TABLE IF NOT EXISTS queued_deployments
(
    id INTEGER PRIMARY KEY autoincrement,
    created_at TIMESTAMP,
    manifest VARCHAR,
    processed BOOLEAN,
    processed_at TIMESTAMP
);
