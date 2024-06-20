CREATE TABLE IF NOT EXISTS cutoff
(
    processedTime TIMESTAMP,
    eslId INTEGER,
    FOREIGN KEY (eslId) REFERENCES event_sourcing_light(eslId)
);
