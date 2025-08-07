SET TRANSACTION ISOLATION LEVEL SERIALIZABLE;
-- create environment_locks table if it doesn't exist
CREATE TABLE IF NOT EXISTS environment_locks
(
    created TIMESTAMP,
    lockid VARCHAR,
    envname VARCHAR,
    metadata VARCHAR,
    PRIMARY KEY(envname, lockid)
);
