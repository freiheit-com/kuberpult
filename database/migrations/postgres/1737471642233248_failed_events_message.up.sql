DO $$
BEGIN
  IF NOT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name = 'event_sourcing_light_failed' AND column_name='reason') THEN
      ALTER TABLE event_sourcing_light_failed
        ADD COLUMN IF NOT EXISTS reason VARCHAR DEFAULT '',
        ADD COLUMN IF NOT EXISTS transformerEslVersion INTEGER
          CONSTRAINT fk_event_sourcing_light_failed_transformer_version
          REFERENCES event_sourcing_light(eslVersion) default 0;
  END IF;
END $$;
