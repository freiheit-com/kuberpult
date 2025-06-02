CREATE TABLE IF NOT EXISTS app_locks_history
(
    created TIMESTAMP,
    lockid VARCHAR,
    envname VARCHAR,
    appname VARCHAR,
    metadata VARCHAR,
    deleted BOOLEAN,
    version INTEGER PRIMARY KEY AUTOINCREMENT
);

INSERT INTO app_locks_history (created, lockid, envname, appname, metadata, deleted)
SELECT created, lockid, envname, appname, metadata, deleted
FROM app_locks
ORDER BY eslversion;

DROP TABLE IF EXISTS app_locks;
DROP TABLE IF EXISTS all_app_locks;

CREATE TABLE IF NOT EXISTS app_locks
(
    created TIMESTAMP,
    lockid VARCHAR,
    envname VARCHAR,
    appname VARCHAR,
    metadata VARCHAR,
    PRIMARY KEY (appname, envname, lockid)
);
