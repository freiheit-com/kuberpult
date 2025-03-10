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

CREATE TABLE IF NOT EXISTS team_locks
(
    created TIMESTAMP,
    lockid VARCHAR,
    envname VARCHAR,
    teamname VARCHAR,
    metadata VARCHAR,
    PRIMARY KEY(teamname, envname, lockid)
);

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