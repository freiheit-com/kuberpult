CREATE TABLE IF NOT EXISTS environments
(
    created TIMESTAMP,
    version BIGINT,
    name VARCHAR(255),
    json VARCHAR(1000000),
    PRIMARY KEY(name, version)
);