CREATE TABLE IF NOT EXISTS event_sourcing_light -- aka ESL
(
    eslId INTEGER PRIMARY KEY autoincrement,
    created TIMESTAMP,
    event_type VARCHAR(255),
    json VARCHAR
);
