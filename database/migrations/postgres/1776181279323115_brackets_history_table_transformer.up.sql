-- 1. Add the column to allow accessing the brackets history by transformer ID
ALTER TABLE IF EXISTS brackets_history
    ADD COLUMN IF NOT EXISTS "source_transformer_esl_id" integer;

-- 2. Add the constraint
DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'fk_source_transformer') THEN
ALTER TABLE brackets_history
    ADD CONSTRAINT fk_source_transformer
        FOREIGN KEY ("source_transformer_esl_id")
            REFERENCES event_sourcing_light (eslversion)
    NOT VALID;
END IF;
END $$;

-- 3. Validate (The "Safe" scan)
-- Even if the column is all NULLs, this confirms it and enables query optimization.
ALTER TABLE brackets_history VALIDATE CONSTRAINT fk_source_transformer;
