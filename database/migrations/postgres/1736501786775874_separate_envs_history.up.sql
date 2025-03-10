-- rename environments table to environments_history if it doesn't exist
DO $$
BEGIN
    IF EXISTS (
        SELECT FROM information_schema.tables 
        WHERE table_schema = 'public' AND table_name = 'environments'
    ) AND NOT EXISTS (
        SELECT FROM information_schema.tables 
        WHERE table_schema = 'public' AND table_name = 'environments_history'
    ) THEN
        ALTER TABLE environments RENAME TO environments_history;
    END IF;
END
$$;

-- create apps table if it doesn't exist
CREATE TABLE IF NOT EXISTS environments 
(
    created TIMESTAMP,
    name VARCHAR(255),
    json VARCHAR,
    applications VARCHAR,
    PRIMARY KEY(name)
);
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1
        FROM information_schema.table_constraints
        WHERE table_name = 'environments'
          AND constraint_type = 'PRIMARY KEY'
    ) THEN
        ALTER TABLE environments
        ADD CONSTRAINT environments_pkey PRIMARY KEY (name);
    END IF;
END $$;
-- insert data into environments table from environments_history table if there's no data inside it
DO $$
BEGIN
    IF EXISTS (
        SELECT FROM information_schema.tables 
        WHERE table_schema = 'public' AND table_name = 'environments'
    ) AND NOT EXISTS (
        SELECT 1 FROM environments LIMIT 1
    ) AND EXISTS (
        SELECT FROM information_schema.tables 
        WHERE table_schema = 'public' AND table_name = 'environments_history'
    ) THEN
        INSERT INTO environments (created, name, json, applications)
        SELECT DISTINCT
            environments_history.created,
            environments_history.name,
            environments_history.json,
            environments_history.applications
        FROM (
            SELECT
                MAX(version) AS latestVersion,
                name
            FROM
                "environments_history"
            GROUP BY
                name) AS latest
        JOIN
            environments_history AS environments_history 
        ON
            latest.latestVersion=environments_history.version
            AND latest.name=environments_history.name;
    END IF;
END
$$;

