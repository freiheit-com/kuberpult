CREATE TABLE IF NOT EXISTS apps_history
(
    created TIMESTAMP,
    appName VARCHAR,
    stateChange VARCHAR,
    metadata VARCHAR,
    version INTEGER PRIMARY KEY AUTOINCREMENT
);

INSERT INTO apps_history (created, appname, stateChange, metadata)
SELECT created, appname, stateChange, metadata 
FROM apps
ORDER BY version;
DROP TABLE IF EXISTS apps;
DROP TABLE IF EXISTS all_apps;
CREATE TABLE IF NOT EXISTS apps
(
    created TIMESTAMP,
    appName VARCHAR,
    stateChange VARCHAR,
    metadata VARCHAR,
    PRIMARY KEY (appname)
);
