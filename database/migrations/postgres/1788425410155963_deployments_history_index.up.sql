CREATE INDEX IF NOT EXISTS idx_deployments_history_app_env_created_version
    ON deployments_history (appname, envname, created DESC, version DESC)
    INCLUDE (releaseversion, revision);
