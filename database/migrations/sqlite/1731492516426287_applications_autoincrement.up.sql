CREATE TABLE IF NOT EXISTS apps_new
(
    created TIMESTAMP,
    appName VARCHAR,
    stateChange VARCHAR,
    metadata VARCHAR,
    version INTEGER PRIMARY KEY AUTOINCREMENT
);

INSERT INTO apps_new (created, appname, statechange, metadata)
SELECT created, appname, statechange, metadata
FROM apps
ORDER BY eslversion;

DROP TABLE IF EXISTS apps;
ALTER TABLE apps_new RENAME TO apps;
