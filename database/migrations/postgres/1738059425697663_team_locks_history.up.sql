SET TRANSACTION ISOLATION LEVEL SERIALIZABLE;
-- rename team_locks table to team_locks_history if it doesn't exist
DO $$
BEGIN
    IF EXISTS (
        SELECT FROM information_schema.tables 
        WHERE table_name = 'team_locks'
    ) AND NOT EXISTS (
        SELECT FROM information_schema.tables 
        WHERE table_name = 'team_locks_history'
    ) THEN
        ALTER TABLE team_locks RENAME TO team_locks_history;
    END IF;
END
$$;