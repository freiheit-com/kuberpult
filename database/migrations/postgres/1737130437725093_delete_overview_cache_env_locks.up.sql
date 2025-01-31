-- We changed the overview structure. As such, we need to recalculate the overview
DO $$ 
BEGIN
    IF EXISTS (SELECT 1 FROM information_schema.tables WHERE table_name = 'overview_cache') THEN
        INSERT INTO overview_cache (timestamp, json) VALUES (now(), '{}');
    END IF;
END $$;