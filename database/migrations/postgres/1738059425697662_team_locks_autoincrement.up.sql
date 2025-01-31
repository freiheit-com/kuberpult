SET TRANSACTION ISOLATION LEVEL SERIALIZABLE;

DO $$ 
BEGIN
    IF EXISTS (SELECT 1 
               FROM information_schema.columns 
               WHERE table_name = 'team_locks' 
                 AND column_name = 'eslversion') THEN
        ALTER TABLE IF EXISTS team_locks ADD COLUMN IF NOT EXISTS version INTEGER;
    END IF;
END $$;
DO $$ 
BEGIN
    IF EXISTS (SELECT 1 
               FROM information_schema.columns 
               WHERE table_name = 'team_locks' 
                 AND column_name = 'eslversion') THEN
        EXECUTE 'WITH ordered_rows AS (
            SELECT eslversion, teamname, lockid, envname, ROW_NUMBER() OVER (ORDER BY eslversion) AS row_num
            FROM team_locks
        )
        UPDATE team_locks
        SET version = ordered_rows.row_num
        FROM ordered_rows
        WHERE team_locks.eslversion = ordered_rows.eslversion AND team_locks.teamname = ordered_rows.teamname AND team_locks.envname = ordered_rows.envname AND team_locks.lockid = ordered_rows.lockid;';
    END IF;
END $$;

DO $$ 
BEGIN
    IF EXISTS (SELECT 1 
               FROM information_schema.columns 
               WHERE table_name = 'team_locks' 
                 AND column_name = 'eslversion') THEN
        CREATE SEQUENCE IF NOT EXISTS team_locks_version_seq OWNED BY team_locks.version;
    END IF;
END $$;

DO $$ 
BEGIN
    IF EXISTS (SELECT 1 
               FROM information_schema.columns 
               WHERE table_name = 'team_locks' 
                 AND column_name = 'eslversion') THEN
        PERFORM setval('team_locks_version_seq', coalesce(max(version), 0) + 1, false) FROM team_locks;
    END IF;
END $$;

DO $$ 
BEGIN
    IF EXISTS (SELECT 1 
               FROM information_schema.columns 
               WHERE table_name = 'team_locks' 
                 AND column_name = 'eslversion') THEN
        ALTER TABLE IF EXISTS team_locks 
        ALTER COLUMN version SET DEFAULT nextval('team_locks_version_seq');
    END IF;
END $$;

DO $$ 
BEGIN
    IF EXISTS (SELECT 1 
               FROM information_schema.columns 
               WHERE table_name = 'team_locks' 
                 AND column_name = 'eslversion') THEN
        ALTER TABLE IF EXISTS team_locks DROP CONSTRAINT IF EXISTS team_locks_pkey;
    END IF;
END $$;

DO $$ 
BEGIN
    IF EXISTS (SELECT 1 
               FROM information_schema.columns 
               WHERE table_name = 'team_locks' 
                 AND column_name = 'eslversion') THEN
        ALTER TABLE IF EXISTS team_locks ADD PRIMARY KEY (version, teamname, envname, lockid);
    END IF;
END $$;

ALTER TABLE IF EXISTS team_locks DROP COLUMN IF EXISTS eslversion;
