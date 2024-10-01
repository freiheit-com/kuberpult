DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name = 'environments' AND column_name='applications') THEN
        ALTER TABLE IF EXISTS environments ADD COLUMN applications VARCHAR;
    END IF;
END $$;