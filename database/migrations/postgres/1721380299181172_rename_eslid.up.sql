IF EXISTS(SELECT *
FROM information_schema.columns
WHERE table_name='cutoff' and column_name='eslId')
THEN
    ALTER TABLE cutoff RENAME COLUMN eslId TO eslVersion;
END IF;

IF EXISTS(SELECT *
FROM information_schema.columns
WHERE table_name='event_sourcing_light' and column_name='eslId')
THEN
    ALTER TABLE event_sourcing_light RENAME COLUMN eslId TO eslVersion;
END IF

IF EXISTS(SELECT *
FROM information_schema.columns
WHERE table_name='event_sourcing_light_failed' and column_name='eslId')
THEN
    ALTER TABLE event_sourcing_light_failed RENAME COLUMN eslId TO eslVersion;
END IF

IF EXISTS(SELECT *
FROM information_schema.columns
WHERE table_name='overview_cache' and column_name='eslId')
THEN
    ALTER TABLE overview_cache RENAME COLUMN eslId TO eslVersion;
END IF