CREATE TABLE new_environments
(
    created TIMESTAMP,
    version BIGINT,
    name VARCHAR(255),
    json VARCHAR(1000000),
    PRIMARY KEY(name, version)
);

INSERT INTO new_environments (created, version, name, json)
SELECT created, version, name, json
FROM environments;

DROP TABLE environments;

ALTER TABLE new_environments RENAME TO environments;