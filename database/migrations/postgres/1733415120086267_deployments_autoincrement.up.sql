ALTER TABLE IF EXISTS deployments ADD COLUMN IF NOT EXISTS version INTEGER;
DO $$ 
BEGIN
    IF EXISTS (SELECT 1 
               FROM information_schema.columns 
               WHERE table_name = 'deployments' 
                 AND column_name = 'eslversion') THEN
        EXECUTE 'WITH ordered_rows AS (
            SELECT eslversion, envname, appname, ROW_NUMBER() OVER (ORDER BY eslversion) AS row_num
            FROM deployments
        )
        UPDATE deployments
        SET version = ordered_rows.row_num
        FROM ordered_rows
        WHERE deployments.eslversion = ordered_rows.eslversion AND deployments.appname = ordered_rows.appname AND deployments.envname = ordered_rows.envname;';
    END IF;
END $$;

CREATE SEQUENCE IF NOT EXISTS deployments_version_seq OWNED BY deployments.version;

SELECT setval('deployments_version_seq', coalesce(max(version), 0) + 1, false) FROM deployments;

ALTER TABLE IF EXISTS deployments
ALTER COLUMN version SET DEFAULT nextval('deployments_version_seq');

ALTER TABLE IF EXISTS deployments DROP CONSTRAINT IF EXISTS deployments_pkey;

ALTER TABLE IF EXISTS deployments ADD PRIMARY KEY (version, appname, envname);

ALTER TABLE IF EXISTS deployments DROP COLUMN IF EXISTS eslversion;
