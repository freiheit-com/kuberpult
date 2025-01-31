
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM event_sourcing_light LIMIT 1
    ) THEN
        INSERT INTO event_sourcing_light (eslversion, created, event_type, json)
        VALUES (0, now(), 'MigrationTransformer', '{"eslVersion":0,"metadata":{"authorEmail":"Migration","authorName":"Migration"}}');
    END IF;
END
$$;
