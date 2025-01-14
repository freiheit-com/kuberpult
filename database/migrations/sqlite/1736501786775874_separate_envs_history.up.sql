CREATE TABLE IF NOT EXISTS environments_history 
(
    created TIMESTAMP,
    name VARCHAR,
    json VARCHAR,
    applications VARCHAR,
    deleted BOOLEAN,
    version INTEGER PRIMARY KEY AUTOINCREMENT
);

INSERT INTO environments_history (created, name, json, applications, deleted)
SELECT created, name, json, applications, deleted 
FROM environments
ORDER BY version;
DROP TABLE IF EXISTS environments;
DROP TABLE IF EXISTS all_environments;
CREATE TABLE IF NOT EXISTS environments
(
    created TIMESTAMP,
    name VARCHAR,
    json VARCHAR,
    applications VARCHAR,
    PRIMARY KEY (name)
);
