CREATE INDEX if not exists idx_releases_history_environments_gin
    ON releases_history USING GIN (environments) WHERE deleted = false;
