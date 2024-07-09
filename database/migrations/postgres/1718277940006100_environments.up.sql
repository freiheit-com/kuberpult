CREATE TABLE IF NOT EXISTS environments
(
    created TIMESTAMP,
    version BIGINT,
    name VARCHAR(255),
    json VARCHAR(255),
    PRIMARY KEY(name, version)
);