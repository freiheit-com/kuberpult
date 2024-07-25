DO $$ BEGIN IF NOT EXISTS (
    SELECT 1
    FROM information_schema.tables
    WHERE table_name = 'app_locks'
) THEN
ALTER TABLE IF EXISTS application_locks
    RENAME TO app_locks;
END IF;
END $$;