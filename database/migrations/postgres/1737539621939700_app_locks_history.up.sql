SET TRANSACTION ISOLATION LEVEL SERIALIZABLE;
-- rename app_locks table to app_locks_history if it doesn't exist
DO $$
BEGIN
    IF EXISTS (
        SELECT FROM information_schema.tables 
        WHERE table_name = 'app_locks'
    ) AND NOT EXISTS (
        SELECT FROM information_schema.tables 
        WHERE table_name = 'app_locks_history'
    ) THEN
        ALTER TABLE app_locks RENAME TO app_locks_history;
    END IF;
END
$$;