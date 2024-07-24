DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM information_schema.tables WHERE table_name = 'commit_events') THEN
        ALTER TABLE IF EXISTS events RENAME TO commit_events;
    END IF;
END $$;
