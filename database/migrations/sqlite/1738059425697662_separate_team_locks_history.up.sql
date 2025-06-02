CREATE TABLE IF NOT EXISTS team_locks_history
(
    created TIMESTAMP,
    lockid VARCHAR,
    envname VARCHAR,
    teamname VARCHAR,
    metadata VARCHAR,
    deleted BOOLEAN,
    version INTEGER PRIMARY KEY AUTOINCREMENT
);

INSERT INTO team_locks_history (created, lockid, envname, teamname, metadata, deleted)
SELECT created, lockid, envname, teamname, metadata, deleted
FROM team_locks
ORDER BY eslversion;

DROP TABLE IF EXISTS team_locks;
DROP TABLE IF EXISTS all_team_locks;

CREATE TABLE IF NOT EXISTS team_locks
(
    created TIMESTAMP,
    lockid VARCHAR,
    envname VARCHAR,
    teamname VARCHAR,
    metadata VARCHAR,
    PRIMARY KEY (teamname, envname, lockid)
);
