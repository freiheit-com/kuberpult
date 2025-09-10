CREATE INDEX if not exists idx_apps_history_app_meta_created_version
    ON apps_history (appname, metadata, created DESC, version DESC);
