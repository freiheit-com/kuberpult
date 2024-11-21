ALTER TABLE IF EXISTS environments ADD COLUMN IF NOT EXISTS row_version INTEGER;
DO $$ 
BEGIN
    IF EXISTS (SELECT 1 
               FROM information_schema.columns 
               WHERE table_name = 'environments' 
                 AND column_name = 'version') THEN
        EXECUTE 'WITH ordered_rows AS (
            SELECT version, name, ROW_NUMBER() OVER (ORDER BY version) AS row_num
            FROM environments
        )
        UPDATE environments
        SET row_version = ordered_rows.row_num
        FROM ordered_rows
        WHERE environments.version = ordered_rows.version AND environments.name = ordered_rows.name;';
    END IF;
END $$;

CREATE SEQUENCE IF NOT EXISTS environments_row_version_seq OWNED BY environments.row_version;

SELECT setval('environments_row_version_seq', coalesce(max(row_version), 0) + 1, false) FROM environments;

ALTER TABLE IF EXISTS environments
ALTER COLUMN row_version SET DEFAULT nextval('environments_row_version_seq');

ALTER TABLE IF EXISTS environments DROP CONSTRAINT IF EXISTS environments_pkey;

ALTER TABLE IF EXISTS environments ADD PRIMARY KEY (row_version, name);

ALTER TABLE IF EXISTS environments DROP COLUMN IF EXISTS version;
