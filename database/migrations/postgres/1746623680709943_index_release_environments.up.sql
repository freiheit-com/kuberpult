DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1
        FROM pg_indexes
        WHERE tablename = 'releases'
          AND indexname = 'releases_environments_idx'
    ) THEN
        CREATE INDEX releases_environments_idx
        ON releases (environments);
    END IF;
END
$$;