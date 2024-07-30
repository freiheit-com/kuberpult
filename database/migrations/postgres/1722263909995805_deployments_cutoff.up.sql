CREATE TABLE IF NOT EXISTS queued_deployments
(
    id SERIAL PRIMARY KEY,
    created_at TIMESTAMP,
    manifest VARCHAR,
    processed BOOLEAN,
    processed_at TIMESTAMP
);
