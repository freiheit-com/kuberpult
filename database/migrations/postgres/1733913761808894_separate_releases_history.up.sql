-- rename releases table to releases_history if it doesn't exist
DO $$
BEGIN
    IF EXISTS (
        SELECT FROM information_schema.tables 
        WHERE table_schema = 'public' AND table_name = 'releases'
    ) AND NOT EXISTS (
        SELECT FROM information_schema.tables 
        WHERE table_schema = 'public' AND table_name = 'releases_history'
    ) THEN
        ALTER TABLE releases RENAME TO releases_history;
    END IF;
END
$$;

-- create releases table if it doesn't exist
CREATE TABLE IF NOT EXISTS releases
(
    releaseVersion INTEGER,
    created TIMESTAMP,
    appName VARCHAR,
    manifests VARCHAR,
    metadata VARCHAR, 
    environments VARCHAR,
    PRIMARY KEY(releaseVersion, appName)
);

-- insert data into releases table from releases_history table if there's no data inside it
DO $$
BEGIN
    IF EXISTS (
        SELECT FROM information_schema.tables 
        WHERE table_schema = 'public' AND table_name = 'releases'
    ) AND NOT EXISTS (
        SELECT 1 FROM releases LIMIT 1
    ) AND EXISTS (
        SELECT FROM information_schema.tables 
        WHERE table_schema = 'public' AND table_name = 'releases_history'
    ) THEN
        INSERT INTO releases (releaseVersion, created, appName, manifests, metadata, environments)
        SELECT DISTINCT
            releases_history.releaseVersion,
            releases_history.created,
            releases_history.appName,
            releases_history.manifests,
            releases_history.metadata,
            releases_history.environments
        FROM (
            SELECT
                MAX(version) AS latestRelease,
                appname,
                releaseversion
            FROM
                "releases_history"
            GROUP BY
                appname, releaseversion) AS latest
        JOIN
            releases_history AS releases_history
        ON
            latest.latestRelease=releases_history.version
            AND latest.releaseVersion=releases_history.releaseVersion
            AND latest.appname=releases_history.appname
        WHERE releases_history.deleted=false;
    END IF;
END
$$;

-- Remove all_releases table
DROP TABLE IF EXISTS all_releases;
