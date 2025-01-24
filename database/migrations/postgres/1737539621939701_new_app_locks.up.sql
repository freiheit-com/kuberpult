SET TRANSACTION ISOLATION LEVEL SERIALIZABLE;
-- insert data into app_locks table from app_locks_history table if there's no data inside it
DO $$
BEGIN
    IF EXISTS (
        SELECT FROM information_schema.tables 
        WHERE table_name = 'app_locks'
    ) AND NOT EXISTS (
        SELECT 1 FROM app_locks LIMIT 1
    ) AND EXISTS (
        SELECT FROM information_schema.tables 
        WHERE table_name = 'app_locks_history'
    ) THEN
        INSERT INTO app_locks (created, lockId, envname, appname, metadata)
        SELECT DISTINCT
            app_locks_history.created,
            app_locks_history.lockId,
            app_locks_history.envname,
            app_locks_history.appname,
            app_locks_history.metadata
        FROM (
            SELECT
                MAX(version) AS latestVersion,
                appname,
                envname,
                lockid
            FROM
                "app_locks_history"
            GROUP BY
                appname, envname, lockid) AS latest
        JOIN
            app_locks_history AS app_locks_history 
        ON
            latest.latestVersion=app_locks_history.version
            AND latest.appname=app_locks_history.appname
            AND latest.envname=app_locks_history.envname
            AND latest.lockid=app_locks_history.lockid
        WHERE deleted=false;
    END IF;
END
$$;
