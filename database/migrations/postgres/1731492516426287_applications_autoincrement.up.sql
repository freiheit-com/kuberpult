ALTER TABLE IF EXISTS apps ADD COLUMN IF NOT EXISTS version INTEGER;
WITH ordered_rows AS (
    SELECT eslversion, appname, ROW_NUMBER() OVER (ORDER BY eslversion) AS row_num
    FROM apps
)
UPDATE apps
SET version = ordered_rows.row_num
FROM ordered_rows
WHERE apps.eslversion = ordered_rows.eslversion AND apps.appname = ordered_rows.appname;

CREATE SEQUENCE IF NOT EXISTS apps_version_seq OWNED BY apps.version;

SELECT setval('apps_version_seq', coalesce(max(version), 0) + 1, false) FROM apps;

ALTER TABLE IF EXISTS apps
ALTER COLUMN version SET DEFAULT nextval('apps_version_seq');

ALTER TABLE IF EXISTS apps DROP CONSTRAINT IF EXISTS apps_pkey;

ALTER TABLE IF EXISTS apps ADD PRIMARY KEY (version, appname);

ALTER TABLE IF EXISTS apps DROP COLUMN IF EXISTS eslversion;
