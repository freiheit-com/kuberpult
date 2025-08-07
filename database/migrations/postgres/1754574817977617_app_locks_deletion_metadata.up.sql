DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name = 'app_locks_history' AND column_name='deletionmetadata') THEN
        ALTER TABLE IF EXISTS app_locks_history ADD COLUMN deletionmetadata varchar;
    END IF;
END $$;