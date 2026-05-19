DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name = 'releases' AND column_name='commitHash') THEN
    ALTER TABLE IF EXISTS releases ADD COLUMN commitHash VARCHAR(64);
END IF;
END $$;

CREATE TABLE IF NOT EXISTS commits_history
(
    commitHash VARCHAR(64) PRIMARY KEY NOT NULL,
    previousCommitHash VARCHAR(64)
);