ALTER TABLE environments
ADD COLUMN IF NOT EXISTS deleted bool
DEFAULT false NOT NULL;