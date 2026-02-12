ALTER TABLE apps ADD COLUMN teamname VARCHAR(255);

UPDATE apps
SET
    teamname = metadata::jsonb ->> 'Team'
WHERE
    metadata IS NOT NULL
    and metadata != '';

COMMENT ON COLUMN apps.metadata IS 'DEPRECATED: Use the "teamname" instead if you want to work with team data.';