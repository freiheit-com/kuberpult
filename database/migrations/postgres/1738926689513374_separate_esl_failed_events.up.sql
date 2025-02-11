-- rename event_sourcing_light_failed table to event_sourcing_light_failed if it doesn't exist
SET TRANSACTION ISOLATION LEVEL SERIALIZABLE;
DO $$
BEGIN
    IF EXISTS (
        SELECT FROM information_schema.tables
        WHERE table_name = 'event_sourcing_light_failed'
    ) AND NOT EXISTS (
        SELECT FROM information_schema.tables
        WHERE table_name = 'event_sourcing_light_failed_history'
    ) THEN
ALTER TABLE event_sourcing_light_failed RENAME TO event_sourcing_light_failed_history;
END IF;
END
$$;

CREATE TABLE IF NOT EXISTS event_sourcing_light_failed(
  created timestamp,
  event_type VARCHAR NOT NULL ,
  json varchar NOT NULL,
  reason varchar,
  transformerEslVersion integer DEFAULT 0,
  PRIMARY KEY(transformerEslVersion),
  CONSTRAINT fk_transformerEslVersion_transformer_id FOREIGN key(transformereslversion) REFERENCES event_sourcing_light(eslversion)
);

-- insert data into event_sourcing_light_failed table from event_sourcing_light_failed_history table if there's no data inside it
DO $$
BEGIN
    IF EXISTS (
        SELECT FROM information_schema.tables
        WHERE table_name = 'event_sourcing_light_failed'
    ) AND NOT EXISTS (
        SELECT 1 FROM event_sourcing_light_failed LIMIT 1
    ) THEN
INSERT INTO event_sourcing_light_failed (created, event_type, json, reason, transformereslversion)
SELECT DISTINCT
    event_sourcing_light_failed_history.created,
    event_sourcing_light_failed_history.event_type,
    event_sourcing_light_failed_history.json,
    event_sourcing_light_failed_history.reason,
    event_sourcing_light_failed_history.transformereslversion
FROM event_sourcing_light_failed_history;
END IF;
END
$$;
