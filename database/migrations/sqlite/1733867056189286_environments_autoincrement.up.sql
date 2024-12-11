CREATE TABLE IF NOT EXISTS environments_new 
(
    version INTEGER PRIMARY KEY AUTOINCREMENT,
    created TIMESTAMP,
    name VARCHAR(255),
    json VARCHAR,
    deleted bool DEFAULT false NOT NULL,
    applications VARCHAR
);

INSERT INTO environments_new (created, name, json, deleted, applications)
SELECT created, name, json, deleted, applications
FROM environments
ORDER BY version;

DROP TABLE IF EXISTS environments;
ALTER TABLE environments_new RENAME TO environments;
