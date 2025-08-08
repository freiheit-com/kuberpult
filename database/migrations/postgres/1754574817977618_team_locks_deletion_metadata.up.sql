DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name = 'team_locks_history' AND column_name='deletionmetadata') THEN
        ALTER TABLE IF EXISTS team_locks_history ADD COLUMN deletionmetadata varchar DEFAULT '{}';
    END IF;
END $$;