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

DROP SEQUENCE IF EXISTS environments_version_seq CASCADE;
CREATE SEQUENCE IF NOT EXISTS environments_version_seq OWNED BY environments.row_version;

SELECT setval('environments_version_seq', coalesce(max(row_version), 0) + 1, false) FROM environments;

ALTER TABLE IF EXISTS environments
ALTER COLUMN row_version SET DEFAULT nextval('environments_version_seq');

DO $$
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
        WHERE conname LIKE 'environments_pkey%'
    LOOP
        EXECUTE cmd;
    END LOOP;
END $$;


ALTER TABLE IF EXISTS environments ADD PRIMARY KEY (row_version, name);

ALTER TABLE IF EXISTS environments DROP COLUMN IF EXISTS version;
ALTER TABLE IF EXISTS environments RENAME COLUMN row_version TO version;
