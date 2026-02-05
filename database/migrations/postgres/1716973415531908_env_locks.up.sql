CREATE TABLE IF NOT EXISTS environment_locks
(
    eslVersion INTEGER, -- internal ID for ESL
    created TIMESTAMP,
    lockID VARCHAR,
    envName VARCHAR,
    metadata VARCHAR,
    deleted boolean,
    PRIMARY KEY(eslVersion, lockID, envName)
);
