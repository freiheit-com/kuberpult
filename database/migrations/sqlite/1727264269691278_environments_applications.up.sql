CREATE TABLE IF NOT EXISTS environments_new
(
    created TIMESTAMP,
    version BIGINT,
    name VARCHAR(255),
    json VARCHAR,
    applications VARCHAR,
    PRIMARY KEY(name, version)
);

INSERT INTO environments_new(created, version, name, json)
SELECT created, version, name, json
FROM environments;

DROP TABLE IF EXISTS environments;
ALTER TABLE environments_new RENAME TO environments;
