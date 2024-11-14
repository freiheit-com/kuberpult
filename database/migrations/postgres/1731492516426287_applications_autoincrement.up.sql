ALTER TABLE IF EXISTS apps ADD COLUMN version INTEGER;
WITH ordered_rows AS (
    SELECT eslversion, appname, ROW_NUMBER() OVER (ORDER BY eslversion) AS row_num
    FROM apps
)
UPDATE apps
SET version = ordered_rows.row_num
FROM ordered_rows
WHERE apps.eslversion = ordered_rows.eslversion AND apps.appname = ordered_rows.appname;

CREATE SEQUENCE apps_version_seq OWNED BY apps.version;

SELECT setval('apps_version_seq', coalesce(max(version), 0) + 1, false) FROM apps;

ALTER TABLE apps
ALTER COLUMN version SET DEFAULT nextval('apps_version_seq');

ALTER TABLE apps DROP CONSTRAINT apps_pkey;

ALTER TABLE apps ADD PRIMARY KEY (version, appname);

ALTER TABLE apps DROP COLUMN eslversion;
