ALTER TABLE environments
ADD COLUMN deleted bool
DEFAULT false NOT NULL;
