SET TRANSACTION ISOLATION LEVEL SERIALIZABLE;

-- insert data into team_locks table from team_locks_history table if there's no data inside it
DO $$
BEGIN
    IF EXISTS (
        SELECT FROM information_schema.tables 
        WHERE table_name = 'team_locks'
    ) AND NOT EXISTS (
        SELECT 1 FROM team_locks LIMIT 1
    ) AND EXISTS (
        SELECT FROM information_schema.tables 
        WHERE table_name = 'team_locks_history'
    ) THEN
        INSERT INTO team_locks (created, lockId, envname, teamname, metadata)
        SELECT DISTINCT
            team_locks_history.created,
            team_locks_history.lockId,
            team_locks_history.envname,
            team_locks_history.teamname,
            team_locks_history.metadata
        FROM (
            SELECT
                MAX(version) AS latestVersion,
                teamname,
                envname,
                lockid
            FROM
                "team_locks_history"
            GROUP BY
                teamname, envname, lockid) AS latest
        JOIN
            team_locks_history AS team_locks_history 
        ON
            latest.latestVersion=team_locks_history.version
            AND latest.teamname=team_locks_history.teamname
            AND latest.envname=team_locks_history.envname
            AND latest.lockid=team_locks_history.lockid
        WHERE deleted=false;
    END IF;
END
$$;
