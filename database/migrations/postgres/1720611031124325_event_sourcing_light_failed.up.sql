CREATE TABLE IF NOT EXISTS event_sourcing_light_failed
(
    eslId SERIAL PRIMARY KEY,
    created TIMESTAMP,
    event_type VARCHAR(255),
    json VARCHAR
);