DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name = 'environment_locks_history' AND column_name='deletion_metadata') THEN
    ALTER TABLE IF EXISTS environment_locks_history ADD COLUMN deletion_metadata varchar;
END IF;
END $$;