CREATE TABLE IF NOT EXISTS event_sourcing_light -- aka ESL
(
    eslId UUID,
    created TIMESTAMP,
    event_type VARCHAR(255),
    json VARCHAR,
    PRIMARY KEY(eslId)
);

