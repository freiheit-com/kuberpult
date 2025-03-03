DO $$ 
BEGIN 
  IF NOT EXISTS (
    SELECT 'fk_commit_events_transformer_id'
    FROM information_schema.table_constraints
    WHERE table_name = 'commit_events'
        AND constraint_name = 'fk_commit_events_transformer_id'
  ) AND EXISTS (SELECT 1 
               FROM information_schema.columns 
               WHERE table_name = 'event_sourcing_light' 
                 AND column_name = 'eslId') THEN
          ALTER TABLE commit_events
          ADD COLUMN IF NOT EXISTS transformerEslId INTEGER CONSTRAINT fk_commit_events_transformer_id REFERENCES event_sourcing_light(eslId) default 0;
  end if;
END $$;

DO $$ 
BEGIN 
  IF NOT EXISTS (
    SELECT 1
    FROM information_schema.columns
    WHERE table_name='commit_events' and column_name='transformereslversion')
  AND NOT EXISTS (SELECT 1
    FROM information_schema.columns
    WHERE table_name='commit_events' and column_name='transformerEslId') THEN
      ALTER TABLE commit_events ADD COLUMN IF NOT EXISTS transformerEslId INTEGER default 0;
  END IF;
END $$;

DO $$ 
BEGIN 
  IF EXISTS (
    SELECT 1
    FROM information_schema.columns
    WHERE table_name='commit_events' and column_name='transformereslversion')
  AND NOT EXISTS (SELECT 1
    FROM information_schema.columns
    WHERE table_name='commit_events' and column_name='transformerEslId') THEN
      ALTER TABLE commit_events RENAME COLUMN transformereslversion TO transformereslid;
  END IF;
END $$;