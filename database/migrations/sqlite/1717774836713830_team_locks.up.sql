CREATE TABLE IF NOT EXISTS team_locks
(
    eslVersion INTEGER, -- internal ID for ESL
    created TIMESTAMP,
    lockID  VARCHAR,
    envName VARCHAR,
    teamName VARCHAR,
    metadata VARCHAR,
    deleted boolean,
    PRIMARY KEY(eslVersion, teamName, envName, lockID)
);