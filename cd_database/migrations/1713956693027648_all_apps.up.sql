CREATE TABLE IF NOT EXISTS all_apps
(
    version BIGINT,
    created TIMESTAMP,
    json VARCHAR(255),
    PRIMARY KEY(version)
);
