CREATE INDEX IF NOT EXISTS deployments_idx  ON deployments (appName, envname);
CREATE INDEX IF NOT EXISTS environments_idx ON environments (name);
