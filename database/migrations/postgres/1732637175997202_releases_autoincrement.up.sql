DO $$ 
BEGIN
    IF EXISTS (SELECT 1 
               FROM information_schema.columns 
               WHERE table_name = 'releases' 
                 AND column_name = 'eslversion') THEN
        ALTER TABLE IF EXISTS releases ADD COLUMN IF NOT EXISTS version INTEGER;
    END IF;
END $$;
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

DO $$ 
BEGIN
    IF EXISTS (SELECT 1 
               FROM information_schema.columns 
               WHERE table_name = 'releases' 
                 AND column_name = 'eslversion') THEN
        CREATE SEQUENCE IF NOT EXISTS releases_version_seq OWNED BY releases.version;
    END IF;
END $$;

DO $$ 
BEGIN
    IF EXISTS (SELECT 1 
               FROM information_schema.columns 
               WHERE table_name = 'releases' 
                 AND column_name = 'eslversion') THEN
        PERFORM setval('releases_version_seq', coalesce(max(version), 0) + 1, false) FROM releases;
    END IF;
END $$;

DO $$ 
BEGIN
    IF EXISTS (SELECT 1 
               FROM information_schema.columns 
               WHERE table_name = 'releases' 
                 AND column_name = 'eslversion') THEN
        ALTER TABLE IF EXISTS releases
        ALTER COLUMN version SET DEFAULT nextval('releases_version_seq');
    END IF;
END $$;

DO $$ 
BEGIN
    IF EXISTS (SELECT 1 
               FROM information_schema.columns 
               WHERE table_name = 'releases' 
                 AND column_name = 'eslversion') THEN
        ALTER TABLE IF EXISTS releases DROP CONSTRAINT IF EXISTS releases_pkey;
    END IF;
END $$;

DO $$ 
BEGIN
    IF EXISTS (SELECT 1 
               FROM information_schema.columns 
               WHERE table_name = 'releases' 
                 AND column_name = 'eslversion') THEN
        ALTER TABLE IF EXISTS releases ADD PRIMARY KEY (version, appname, releaseversion);
    END IF;
END $$;

ALTER TABLE IF EXISTS releases DROP COLUMN IF EXISTS eslversion;
