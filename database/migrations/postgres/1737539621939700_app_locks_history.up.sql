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

-- create app_locks table if it doesn't exist
CREATE TABLE IF NOT EXISTS app_locks
(
    created TIMESTAMP,
    lockid VARCHAR,
    envname VARCHAR,
    appname VARCHAR,
    metadata VARCHAR,
    PRIMARY KEY(appname, envname, lockid)
);

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