CREATE TABLE IF NOT EXISTS cutoff
(
    processedTime TIMESTAMP,
    eslId SERIAL,
    FOREIGN KEY (eslId) REFERENCES event_sourcing_light(eslId)
);
