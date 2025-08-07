SET TRANSACTION ISOLATION LEVEL SERIALIZABLE;
-- rename environment_locks table to environment_locks_history if it doesn't exist
DO $$
BEGIN
    IF EXISTS (
        SELECT FROM information_schema.tables 
        WHERE table_name = 'environment_locks'
    ) AND NOT EXISTS (
        SELECT FROM information_schema.tables 
        WHERE table_name = 'environment_locks_history'
    ) THEN
        ALTER TABLE environment_locks RENAME TO environment_locks_history;
    END IF;
END
$$;