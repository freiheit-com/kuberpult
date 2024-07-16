CREATE TABLE IF NOT EXISTS overview_cache
(
    eslId INTEGER PRIMARY KEY,
    timestamp TIMESTAMP,
    json VARCHAR
);