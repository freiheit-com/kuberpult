-- rename apps table to apps_history if it doesn't exist
DO $$
BEGIN
    IF EXISTS (
        SELECT FROM information_schema.tables 
        WHERE table_schema = 'public' AND table_name = 'apps'
    ) AND NOT EXISTS (
        SELECT FROM information_schema.tables 
        WHERE table_schema = 'public' AND table_name = 'apps_history'
    ) THEN
        ALTER TABLE apps RENAME TO apps_history;
    END IF;
END
$$;

-- create apps table if it doesn't exist
CREATE TABLE IF NOT EXISTS apps 
(
    created TIMESTAMP,
    appName VARCHAR,
    stateChange VARCHAR,
    metadata VARCHAR,
    PRIMARY KEY(appName)
);

-- insert data into apps table from apps_history table if there's no data inside it
DO $$
BEGIN
    IF EXISTS (
        SELECT FROM information_schema.tables 
        WHERE table_schema = 'public' AND table_name = 'apps'
    ) AND NOT EXISTS (
        SELECT 1 FROM apps LIMIT 1
    ) AND EXISTS (
        SELECT FROM information_schema.tables 
        WHERE table_schema = 'public' AND table_name = 'apps_history'
    ) THEN
        INSERT INTO apps (created, appName, stateChange, metadata)
        SELECT DISTINCT
            apps_history.created,
            apps_history.appName,
            apps_history.stateChange,
            apps_history.metadata
        FROM (
            SELECT
                MAX(version) AS latestVersion,
                appname
            FROM
                "apps_history"
            GROUP BY
                appname) AS latest
        JOIN
            apps_history AS apps_history 
        ON
            latest.latestVersion=apps_history.version
            AND latest.appname=apps_history.appname;
    END IF;
END
$$;

-- Remove all_apps table
DROP TABLE IF EXISTS all_apps;
