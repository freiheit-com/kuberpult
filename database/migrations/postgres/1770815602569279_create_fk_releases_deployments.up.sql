DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 
        FROM pg_constraint 
        WHERE conname = 'fk_releases_deployments'
    ) THEN
        ALTER TABLE deployments
        ADD CONSTRAINT fk_releases_deployments FOREIGN KEY (
            appname,
            releaseVersion,
            revision
        ) REFERENCES releases (
            appname,
            releaseVersion,
            revision
        ) NOT VALID;
    END IF;
END $$;