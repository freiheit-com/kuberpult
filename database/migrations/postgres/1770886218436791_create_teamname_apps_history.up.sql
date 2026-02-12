ALTER TABLE apps_history ADD COLUMN teamname VARCHAR(255);

UPDATE apps_history
SET
    teamname = metadata::jsonb ->> 'Team'
WHERE
    metadata IS NOT NULL
    and metadata != '';

COMMENT ON COLUMN apps_history.metadata IS 'DEPRECATED: Use the "teamname" instead if you want to work with team data.';