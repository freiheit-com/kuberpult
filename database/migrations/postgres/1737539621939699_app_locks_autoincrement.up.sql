DO $$ 
BEGIN
    IF EXISTS (SELECT 1 
               FROM information_schema.columns 
               WHERE table_name = 'app_locks' 
                 AND column_name = 'eslversion') THEN
        ALTER TABLE IF EXISTS app_locks ADD COLUMN IF NOT EXISTS version INTEGER;
    END IF;
END $$;
DO $$ 
BEGIN
    IF EXISTS (SELECT 1 
               FROM information_schema.columns 
               WHERE table_name = 'app_locks' 
                 AND column_name = 'eslversion') THEN
        EXECUTE 'WITH ordered_rows AS (
            SELECT eslversion, appname, lockid, envname, ROW_NUMBER() OVER (ORDER BY eslversion) AS row_num
            FROM app_locks
        )
        UPDATE app_locks
        SET version = ordered_rows.row_num
        FROM ordered_rows
        WHERE app_locks.eslversion = ordered_rows.eslversion AND app_locks.appname = ordered_rows.appname AND app_locks.envname = ordered_rows.envname AND app_locks.lockid = ordered_rows.lockid;';
    END IF;
END $$;

DO $$ 
BEGIN
    IF EXISTS (SELECT 1 
               FROM information_schema.columns 
               WHERE table_name = 'app_locks' 
                 AND column_name = 'eslversion') THEN
        CREATE SEQUENCE IF NOT EXISTS app_locks_version_seq OWNED BY app_locks.version;
    END IF;
END $$;

DO $$ 
BEGIN
    IF EXISTS (SELECT 1 
               FROM information_schema.columns 
               WHERE table_name = 'app_locks' 
                 AND column_name = 'eslversion') THEN
        PERFORM setval('app_locks_version_seq', coalesce(max(version), 0) + 1, false) FROM app_locks;
    END IF;
END $$;

DO $$ 
BEGIN
    IF EXISTS (SELECT 1 
               FROM information_schema.columns 
               WHERE table_name = 'app_locks' 
                 AND column_name = 'eslversion') THEN
        ALTER TABLE IF EXISTS app_locks 
        ALTER COLUMN version SET DEFAULT nextval('app_locks_version_seq');
    END IF;
END $$;

DO $$ 
BEGIN
    IF EXISTS (SELECT 1 
               FROM information_schema.columns 
               WHERE table_name = 'app_locks' 
                 AND column_name = 'eslversion') THEN
        ALTER TABLE IF EXISTS app_locks DROP CONSTRAINT IF EXISTS app_locks_pkey;
        ALTER TABLE IF EXISTS app_locks DROP CONSTRAINT IF EXISTS application_locks_pkey;
    END IF;
END $$;

DO $$ 
BEGIN
    IF EXISTS (SELECT 1 
               FROM information_schema.columns 
               WHERE table_name = 'app_locks' 
                 AND column_name = 'eslversion') THEN
        ALTER TABLE IF EXISTS app_locks ADD PRIMARY KEY (version, appname, envname, lockid);
    END IF;
END $$;

ALTER TABLE IF EXISTS app_locks DROP COLUMN IF EXISTS eslversion;
