ALTER TABLE IF EXISTS releases ADD COLUMN IF NOT EXISTS version INTEGER;
DO $$ 
BEGIN
    IF EXISTS (SELECT 1 
               FROM information_schema.columns 
               WHERE table_name = 'releases' 
                 AND column_name = 'eslversion') THEN
        EXECUTE 'WITH ordered_rows AS (
            SELECT eslversion, releaseversion, appname, ROW_NUMBER() OVER (ORDER BY eslversion) AS row_num
            FROM releases
        )
        UPDATE releases
        SET version = ordered_rows.row_num
        FROM ordered_rows
        WHERE releases.eslversion = ordered_rows.eslversion AND releases.appname = ordered_rows.appname AND releases.releaseversion = ordered_rows.releaseversion;';
    END IF;
END $$;

CREATE SEQUENCE IF NOT EXISTS releases_version_seq OWNED BY releases.version;

SELECT setval('releases_version_seq', coalesce(max(version), 0) + 1, false) FROM releases;

ALTER TABLE IF EXISTS releases
ALTER COLUMN version SET DEFAULT nextval('releases_version_seq');
DO $
DECLARE
    cmd TEXT;
BEGIN
    FOR cmd IN
        SELECT format(
            'ALTER TABLE %I DROP CONSTRAINT %I;',
            relname, conname
        )
        FROM pg_constraint c
        JOIN pg_class t ON c.conrelid = t.oid
        WHERE conname LIKE 'releases_pkey%'
    LOOP
        EXECUTE cmd;
    END LOOP;
END $$;


ALTER TABLE IF EXISTS releases ADD PRIMARY KEY (version, appname, releaseversion);

ALTER TABLE IF EXISTS releases DROP COLUMN IF EXISTS eslversion;
