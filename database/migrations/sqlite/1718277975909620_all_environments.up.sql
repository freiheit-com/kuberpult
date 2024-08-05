CREATE TABLE IF NOT EXISTS all_environments
(
    created TIMESTAMP,
    version BIGINT,
    json VARCHAR(255),
    PRIMARY KEY(version)
);