CREATE TABLE IF NOT EXISTS all_env_locks
(
    version BIGINT,
    created TIMESTAMP,
    environment VARCHAR(255),
    json VARCHAR(255),
    PRIMARY KEY(version, environment)
);