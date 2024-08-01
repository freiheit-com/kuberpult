CREATE TABLE IF NOT EXISTS queued_deployments
(
    id SERIAL PRIMARY KEY,
    created_at TIMESTAMP NOT NULL,
    manifest VARCHAR NOT NULL,
    processed BOOLEAN NOT NULL,
    processed_at TIMESTAMP
);
