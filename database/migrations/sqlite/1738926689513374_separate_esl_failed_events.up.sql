CREATE TABLE IF NOT EXISTS event_sourcing_light_failed_history
(
    eslVersion INTEGER PRIMARY KEY AUTOINCREMENT,
    created timestamp,
    event_type VARCHAR NOT NULL ,
    json varchar NOT NULL,
    reason varchar,
    transformerEslVersion integer DEFAULT 0,
    CONSTRAINT fk_transformerEslVersion_transformer_id FOREIGN key(transformereslversion) REFERENCES event_sourcing_light(eslversion)
);

INSERT INTO event_sourcing_light_failed_history (eslversion, created, event_type, json, reason, transformerEslVersion)
SELECT eslVersion, created, event_type, json, reason, transformerEslVersion
FROM event_sourcing_light_failed
ORDER BY created;

DROP TABLE IF EXISTS event_sourcing_light_failed;

CREATE TABLE IF NOT EXISTS event_sourcing_light_failed
(
    created TIMESTAMP,
    event_type VARCHAR,
    json VARCHAR,
    reason VARCHAR,
    transformerEslVersion VARCHAR,
    PRIMARY KEY (transformerEslVersion)
);
