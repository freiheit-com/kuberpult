CREATE TABLE IF NOT EXISTS cutoff_new
(
    processedTime TIMESTAMP,
    eslVersion INTEGER,
    FOREIGN KEY (eslversion) REFERENCES event_sourcing_light(eslId)
);
INSERT INTO cutoff_new (processedTime, eslVersion)
SELECT processedTime, eslId
FROM cutoff;
DROP TABLE IF EXISTS cutoff;
ALTER TABLE cutoff_new RENAME TO cutoff;

CREATE TABLE IF NOT EXISTS event_sourcing_light_new 
(
    eslVersion INTEGER PRIMARY KEY autoincrement,
    created TIMESTAMP,
    event_type VARCHAR(255),
    json VARCHAR
);
INSERT INTO event_sourcing_light_new (eslVersion, created, event_type, json)
SELECT eslId, created, event_type, json
FROM event_sourcing_light;
DROP TABLE IF EXISTS event_sourcing_light;
ALTER TABLE event_sourcing_light_new RENAME TO event_sourcing_light;

CREATE TABLE IF NOT EXISTS event_sourcing_light_failed_new
(
    eslVersion INTEGER PRIMARY KEY autoincrement,
    created TIMESTAMP NOT NULL,
    event_type VARCHAR(255) NOT NULL,
    json VARCHAR NOT NULL
);
INSERT INTO event_sourcing_light_failed_new (eslVersion, created, event_type, json)
SELECT eslId, created, event_type, json
FROM event_sourcing_light_failed;
DROP TABLE IF EXISTS event_sourcing_light_failed;
ALTER TABLE event_sourcing_light_failed_new RENAME TO event_sourcing_light_failed;

CREATE TABLE IF NOT EXISTS overview_cache_new
(
    eslVersion INTEGER PRIMARY KEY,
    timestamp TIMESTAMP,
    json VARCHAR
);
INSERT INTO overview_cache_new(eslId, timestamp, json)
SELECT eslVersion, timestamp, json
FROM overview_cache;
DROP TABLE IF EXISTS overview_cache;
ALTER TABLE overview_cache_new RENAME TO overview_cache;
