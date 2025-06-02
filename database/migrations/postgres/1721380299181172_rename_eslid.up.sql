DO $$
BEGIN
  IF EXISTS(SELECT *
    FROM information_schema.columns
    WHERE table_name='cutoff' and column_name='eslid')
  THEN
      ALTER TABLE cutoff RENAME COLUMN eslid TO eslVersion;
  END IF;
END $$;

DO $$
BEGIN
  IF EXISTS(SELECT *
    FROM information_schema.columns
    WHERE table_name='event_sourcing_light' and column_name='eslid')
  THEN
      ALTER TABLE event_sourcing_light RENAME COLUMN eslid TO eslVersion;
  END IF;
END $$;

DO $$
BEGIN
  IF EXISTS(SELECT *
    FROM information_schema.columns
    WHERE table_name='event_sourcing_light_failed' and column_name='eslid')
  THEN
      ALTER TABLE event_sourcing_light_failed RENAME COLUMN eslid TO eslVersion;
  END IF;
END $$;

DO $$
BEGIN
  IF EXISTS(SELECT *
    FROM information_schema.columns
    WHERE table_name='overview_cache' and column_name='eslid')
  THEN
      ALTER TABLE overview_cache RENAME COLUMN eslid TO eslVersion;
  END IF;
END $$;

DO $$
BEGIN
  IF EXISTS(SELECT *
    FROM information_schema.columns
    WHERE table_name='commit_events' and column_name='transformereslid')
  AND NOT EXISTS(SELECT 1
    FROM information_schema.columns
    WHERE table_name='commit_events' and column_name='transformereslversion')
  THEN
      ALTER TABLE commit_events RENAME COLUMN transformereslid TO transformereslVersion;
  END IF;
END $$;

DO $$
BEGIN
  IF EXISTS(SELECT *
    FROM information_schema.columns
    WHERE table_name='deployments' and column_name='transformereslid')
  THEN
      ALTER TABLE deployments RENAME COLUMN transformereslid TO transformereslVersion;
  END IF;
END $$;