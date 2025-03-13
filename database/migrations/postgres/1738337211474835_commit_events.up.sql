DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM commit_events LIMIT 1
    ) THEN
        INSERT INTO commit_events (uuid, timestamp, commitHash, eventType, json, transformereslVersion)
        VALUES ('00000000-0000-0000-0000-000000000000', now(), '0000000000000000000000000000000000000000', 'db-migration', '{}', 0);
    END IF;
END
$$;