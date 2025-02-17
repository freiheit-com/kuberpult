
DO $$
BEGIN
    IF EXISTS (SELECT 1 
               FROM information_schema.columns 
               WHERE table_name = 'apps' 
                 AND column_name = 'eslversion') THEN
        ALTER TABLE IF EXISTS apps ADD COLUMN IF NOT EXISTS version INTEGER;
    END IF;
END $$;
DO $$ 
BEGIN
    IF EXISTS (SELECT 1 
               FROM information_schema.columns 
               WHERE table_name = 'apps' 
                 AND column_name = 'eslversion') THEN
        EXECUTE 'WITH ordered_rows AS (
            SELECT eslversion, appname, ROW_NUMBER() OVER (ORDER BY eslversion) AS row_num
            FROM apps
        )
        UPDATE apps
        SET version = ordered_rows.row_num
        FROM ordered_rows
        WHERE apps.eslversion = ordered_rows.eslversion AND apps.appname = ordered_rows.appname;';
    END IF;
END $$;

DO $$ 
BEGIN
    IF EXISTS (SELECT 1 
               FROM information_schema.columns 
               WHERE table_name = 'apps' 
                 AND column_name = 'eslversion') THEN
        CREATE SEQUENCE IF NOT EXISTS apps_version_seq OWNED BY apps.version;
    END IF;
END $$;

DO $$ 
BEGIN
    IF EXISTS (SELECT 1 
               FROM information_schema.columns 
               WHERE table_name = 'apps' 
                 AND column_name = 'eslversion') THEN
        PERFORM setval('apps_version_seq', coalesce(max(version), 0) + 1, false) FROM apps;
    END IF;
END $$;

DO $$ 
BEGIN
    IF EXISTS (SELECT 1 
               FROM information_schema.columns 
               WHERE table_name = 'apps' 
                 AND column_name = 'eslversion') THEN
        ALTER TABLE IF EXISTS apps
        ALTER COLUMN version SET DEFAULT nextval('apps_version_seq');
    END IF;
END $$;

DO $$ 
BEGIN
    IF EXISTS (SELECT 1 
               FROM information_schema.columns 
               WHERE table_name = 'apps' 
                 AND column_name = 'eslversion') THEN
        ALTER TABLE IF EXISTS apps DROP CONSTRAINT IF EXISTS apps_pkey;
    END IF;
END $$;

DO $$ 
BEGIN
    IF EXISTS (SELECT 1 
               FROM information_schema.columns 
               WHERE table_name = 'apps' 
                 AND column_name = 'eslversion') THEN
        ALTER TABLE IF EXISTS apps ADD PRIMARY KEY (version, appname);
    END IF;
END $$;

DO $$ 
BEGIN
    IF EXISTS (SELECT 1 
               FROM information_schema.columns 
               WHERE table_name = 'apps' 
                 AND column_name = 'eslversion') THEN
        ALTER TABLE IF EXISTS apps DROP COLUMN IF EXISTS eslversion;
    END IF;
END $$;
