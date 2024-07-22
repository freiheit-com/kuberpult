ALTER TABLE event_sourcing_light_failed
    ALTER COLUMN created SET NOT NULL,
    ALTER COLUMN event_type SET NOT NULL,
    ALTER COLUMN json SET NOT NULL;