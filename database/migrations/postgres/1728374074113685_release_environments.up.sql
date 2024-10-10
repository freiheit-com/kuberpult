DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name = 'releases' AND column_name='environments') THEN
        ALTER TABLE IF EXISTS releases ADD COLUMN environments VARCHAR;
    END IF;
END $$;