-- This index was created for the manifest-export, see DBSelectAppsWithReleasesAtTimestamp:
-- the key columns match its DISTINCT ON / ORDER BY exactly,
-- and the INCLUDE columns make the scan index-only.
CREATE INDEX IF NOT EXISTS idx_releases_history_app_release_revision_version
    ON releases_history (appname, releaseversion, revision, version DESC)
    INCLUDE (created, deleted, environments);
