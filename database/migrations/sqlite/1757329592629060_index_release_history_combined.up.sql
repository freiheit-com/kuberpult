CREATE INDEX IF NOT EXISTS idx_releases_history_app_release_created_version
    ON releases_history (appname, releaseversion, created DESC, version DESC);
