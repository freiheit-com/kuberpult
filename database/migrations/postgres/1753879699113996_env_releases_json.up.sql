ALTER TABLE releases
ALTER COLUMN environments TYPE jsonb
USING environments::jsonb;
