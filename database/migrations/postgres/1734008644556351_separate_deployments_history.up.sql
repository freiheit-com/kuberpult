-- rename releases table to releases_history if it doesn't exist
DO $$
BEGIN
    IF EXISTS (
        SELECT FROM information_schema.tables 
        WHERE table_schema = 'public' AND table_name = 'deployments'
    ) AND NOT EXISTS (
        SELECT FROM information_schema.tables 
        WHERE table_schema = 'public' AND table_name = 'deployments_history'
    ) THEN
        ALTER TABLE deployments RENAME TO deployments_history;
    END IF;
END
$$;
-- create releases table if it doesn't exist
DROP INDEX IF EXISTS deployments_idx;
DROP INDEX IF EXISTS deployments_version_idx;
CREATE TABLE IF NOT EXISTS deployments(
    created timestamp without time zone,
    releaseversion bigint,
    appname varchar NOT NULL,
    envname varchar NOT NULL,
    metadata varchar,
    transformereslversion integer DEFAULT 0,
    PRIMARY KEY(appname,envname),
    CONSTRAINT fk_deployments_transformer_id FOREIGN key(transformereslversion) REFERENCES event_sourcing_light(eslversion)
);
CREATE INDEX IF NOT EXISTS deployments_idx ON deployments USING btree ("appname","envname");
CREATE INDEX IF NOT EXISTS deployments_version_idx ON deployments USING btree ("releaseversion","appname","envname");

-- insert data into releases table from releases_history table if there's no data inside it
DO $$
BEGIN
    IF EXISTS (
        SELECT FROM information_schema.tables 
        WHERE table_schema = 'public' AND table_name = 'deployments'
    ) AND NOT EXISTS (
        SELECT 1 FROM deployments LIMIT 1
    ) THEN
        INSERT INTO deployments (releaseVersion, created, appName, envname, metadata, transformereslversion)
        SELECT DISTINCT
            deployments_history.releaseVersion,
            deployments_history.created,
            deployments_history.appName,
            deployments_history.envname,
            deployments_history.metadata,
            deployments_history.transformereslversion
        FROM (
            SELECT
                MAX(version) AS latestDeployment,
                appname,
                envname
            FROM
                "deployments_history"
            GROUP BY
                appname, envname) AS latest
        JOIN
            deployments_history AS deployments_history
        ON
            latest.latestDeployment=deployments_history.version
            AND latest.envname=deployments_history.envname
            AND latest.appname=deployments_history.appname;
    END IF;
END
$$;

-- Remove all_releases table
DROP TABLE IF EXISTS all_deployments;
