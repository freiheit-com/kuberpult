SET TRANSACTION ISOLATION LEVEL SERIALIZABLE;
-- create team_locks table if it doesn't exist
CREATE TABLE IF NOT EXISTS team_locks
(
    created TIMESTAMP,
    lockid VARCHAR,
    envname VARCHAR,
    teamname VARCHAR,
    metadata VARCHAR,
    PRIMARY KEY(teamname, envname, lockid)
);
