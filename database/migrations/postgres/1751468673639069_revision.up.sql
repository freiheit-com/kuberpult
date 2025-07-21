DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name = 'releases' AND column_name='revision') THEN
    ALTER TABLE IF EXISTS releases ADD COLUMN revision int NOT NULL DEFAULT 0;
    ALTER TABLE releases DROP CONSTRAINT releases_pkey1;
    ALTER TABLE releases ADD PRIMARY KEY(releaseVersion, appName, revision);
END IF;
END $$;

DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name = 'releases_history' AND column_name='revision') THEN
    ALTER TABLE IF EXISTS releases_history ADD COLUMN revision int NOT NULL DEFAULT 0;
    ALTER TABLE releases_history DROP CONSTRAINT releases_pkey;
    ALTER TABLE releases_history ADD PRIMARY KEY (version, releaseVersion, revision);
END IF;
END $$;

DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name = 'deployments' AND column_name='revision') THEN
    ALTER TABLE IF EXISTS deployments ADD COLUMN revision int NOT NULL DEFAULT 0;
END IF;
END $$;

DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name = 'deployments_history' AND column_name='revision') THEN
    ALTER TABLE IF EXISTS deployments_history ADD COLUMN revision int NOT NULL DEFAULT 0;
END IF;
END $$;

DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name = 'deployment_attempts' AND column_name='revision') THEN
ALTER TABLE IF EXISTS deployment_attempts ADD COLUMN revision int NOT NULL DEFAULT 0;
END IF;
END $$;