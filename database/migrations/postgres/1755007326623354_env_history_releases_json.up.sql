ALTER TABLE releases_history
ALTER COLUMN environments TYPE jsonb
USING environments::jsonb;
