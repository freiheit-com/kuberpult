DO $$
        ALTER TABLE IF EXISTS event_sourcing_light ADD COLUMN IF NOT EXISTS ddspan JSONB;
END $$;
