SET TRANSACTION ISOLATION LEVEL SERIALIZABLE;

-- insert data into environment_locks table from environment_locks_history table if there's no data inside it
DO $$
BEGIN
    IF EXISTS (
        SELECT FROM information_schema.tables 
        WHERE table_name = 'environment_locks'
    ) AND NOT EXISTS (
        SELECT 1 FROM environment_locks LIMIT 1
    ) AND EXISTS (
        SELECT FROM information_schema.tables 
        WHERE table_name = 'environment_locks_history'
    ) THEN
        INSERT INTO environment_locks (created, lockId, envname, metadata)
        SELECT DISTINCT
            environment_locks_history.created,
            environment_locks_history.lockId,
            environment_locks_history.envname,
            environment_locks_history.metadata
        FROM (
            SELECT
                MAX(version) AS latestVersion,
                envname,
                lockid
            FROM
                "environment_locks_history"
            GROUP BY
                envname, lockid) AS latest
        JOIN
            environment_locks_history AS environment_locks_history
        ON
            latest.latestVersion=environment_locks_history.version
            AND latest.envname=environment_locks_history.envname
            AND latest.lockid=environment_locks_history.lockid
        WHERE deleted=false;
    END IF;
END
$$;
