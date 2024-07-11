CREATE TABLE IF NOT EXISTS event_sourcing_light_failed
(
    eslId INTEGER PRIMARY KEY autincrement,
    created TIMESTAMP,
    event_type VARCHAR(255),
    json VARCHAR
);