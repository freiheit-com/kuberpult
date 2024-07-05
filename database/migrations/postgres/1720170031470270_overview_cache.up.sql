CREATE TABLE IF NOT EXISTS overview_cache
(
    eslId SERIAL PRIMARY KEY,
    timestamp TIMESTAMP,
    blob VARCHAR
);