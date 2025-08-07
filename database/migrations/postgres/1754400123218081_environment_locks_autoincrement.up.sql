SET TRANSACTION ISOLATION LEVEL SERIALIZABLE;

DO $$ 
BEGIN
    IF EXISTS (SELECT 1 
               FROM information_schema.columns 
               WHERE table_name = 'environment_locks'
                 AND column_name = 'eslversion') THEN
        ALTER TABLE IF EXISTS environment_locks ADD COLUMN IF NOT EXISTS version INTEGER;
    END IF;
END $$;
DO $$ 
BEGIN
    IF EXISTS (SELECT 1 
               FROM information_schema.columns 
               WHERE table_name = 'environment_locks'
                 AND column_name = 'eslversion') THEN
        EXECUTE 'WITH ordered_rows AS (
            SELECT eslversion, lockid, envname, ROW_NUMBER() OVER (ORDER BY eslversion) AS row_num
            FROM environment_locks
        )
        UPDATE environment_locks
        SET version = ordered_rows.row_num
        FROM ordered_rows
        WHERE environment_locks.eslversion = ordered_rows.eslversion AND environment_locks.envname = ordered_rows.envname AND environment_locks.lockid = ordered_rows.lockid;';
    END IF;
END $$;

DO $$ 
BEGIN
    IF EXISTS (SELECT 1 
               FROM information_schema.columns 
               WHERE table_name = 'environment_locks'
                 AND column_name = 'eslversion') THEN
        CREATE SEQUENCE IF NOT EXISTS environment_locks_version_seq OWNED BY environment_locks.version;
    END IF;
END $$;

DO $$ 
BEGIN
    IF EXISTS (SELECT 1 
               FROM information_schema.columns 
               WHERE table_name = 'environment_locks'
                 AND column_name = 'eslversion') THEN
        PERFORM setval('environment_locks_version_seq', coalesce(max(version), 0) + 1, false) FROM environment_locks;
    END IF;
END $$;

DO $$ 
BEGIN
    IF EXISTS (SELECT 1 
               FROM information_schema.columns 
               WHERE table_name = 'environment_locks'
                 AND column_name = 'eslversion') THEN
        ALTER TABLE IF EXISTS environment_locks
        ALTER COLUMN version SET DEFAULT nextval('environment_locks_version_seq');
    END IF;
END $$;

DO $$ 
BEGIN
    IF EXISTS (SELECT 1 
               FROM information_schema.columns 
               WHERE table_name = 'environment_locks'
                 AND column_name = 'eslversion') THEN
        ALTER TABLE IF EXISTS environment_locks DROP CONSTRAINT IF EXISTS environment_locks_pkey;
    END IF;
END $$;

DO $$ 
BEGIN
    IF EXISTS (SELECT 1 
               FROM information_schema.columns 
               WHERE table_name = 'environment_locks'
                 AND column_name = 'eslversion') THEN
        ALTER TABLE IF EXISTS environment_locks ADD PRIMARY KEY (version, envname, lockid);
    END IF;
END $$;

ALTER TABLE IF EXISTS environment_locks DROP COLUMN IF EXISTS eslversion;
