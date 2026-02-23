DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 
        FROM pg_constraint 
        WHERE conname = 'releaseversion_not_null'
    ) THEN
        ALTER TABLE deployments
        ADD CONSTRAINT releaseversion_not_null CHECK (releaseversion IS NOT NULL) NOT VALID;
    END IF;
END $$;