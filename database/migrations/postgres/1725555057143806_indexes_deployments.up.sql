CREATE INDEX IF NOT EXISTS deployments_version_idx  ON deployments (appName, envname, releaseversion);
