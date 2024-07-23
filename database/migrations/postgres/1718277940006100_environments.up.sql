CREATE TABLE IF NOT EXISTS environments
(
    created TIMESTAMP,
    version BIGINT,
    name VARCHAR,
    json VARCHAR(255),
    PRIMARY KEY(name, version)
);