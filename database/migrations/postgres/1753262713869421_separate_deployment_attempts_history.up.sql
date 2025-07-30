CREATE TABLE IF NOT EXISTS deployment_attempts_history
(
    eslId SERIAL,
    created TIMESTAMP NOT NULL,
    envname VARCHAR NOT NULL,
    appname VARCHAR NOT NULL,
    releaseversion INTEGER,
    revision VARCHAR
);
CREATE INDEX IF NOT EXISTS deployment_attempts_history_env_app ON deployment_attempts_history (envname, appname);

INSERT INTO deployment_attempts_history (created, envname, appname, releaseversion, revision)
SELECT created, envname, appname, queuedreleaseversion, revision
FROM deployment_attempts
ORDER BY created, eslVersion;

CREATE TABLE IF NOT EXISTS deployment_attempts_latest
(
    created TIMESTAMP NOT NULL,
    envname VARCHAR NOT NULL,
    appname VARCHAR NOT NULL,
    releaseversion INTEGER NOT NULL,
    revision VARCHAR
);

CREATE UNIQUE INDEX IF NOT EXISTS deployment_attempts_latest_env_app ON deployment_attempts_latest (envname, appname);

INSERT INTO deployment_attempts_latest (created, envname, appname, releaseversion, revision)
SELECT
  created, envname, appname, releaseversion, revision
FROM (
  SELECT
    ROW_NUMBER() OVER (PARTITION BY appname, envname ORDER BY eslId DESC) AS r,
    history.created, history.envname, history.appname, history.releaseversion, history.revision
  FROM
    deployment_attempts_history AS history) latestByAppEnv
WHERE
  latestByAppEnv.r <= 1
  AND releaseversion IS NOT NULL;
