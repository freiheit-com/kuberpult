CREATE TABLE IF NOT EXISTS event_sourcing_light_failed_new
(
    eslVersion INTEGER PRIMARY KEY autoincrement,
    created TIMESTAMP NOT NULL,
    event_type VARCHAR(255) NOT NULL,
    json VARCHAR NOT NULL,
    reason VARCHAR NOT NULL,
    transformerEslVersion INTEGER,
    FOREIGN KEY(transformerEslVersion) REFERENCES event_sourcing_light(eslVersion)
);
INSERT INTO event_sourcing_light_failed_new (eslVersion, created, event_type, json, reason, transformerEslVersion)
SELECT eslVersion, created, event_type, json, '' AS reason, 0 AS transformerEslVersion
FROM event_sourcing_light_failed;
DROP TABLE IF EXISTS event_sourcing_light_failed;
ALTER TABLE event_sourcing_light_failed_new RENAME TO event_sourcing_light_failed;
