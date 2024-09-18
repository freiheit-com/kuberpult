
DO $$
  BEGIN
    BEGIN
      ALTER TABLE deployments ADD COLUMN appEnvRelease VARCHAR NULL;
  EXCEPTION
      WHEN duplicate_column THEN RAISE NOTICE 'column <column_name> already exists in <table_name>.';
    END;
  END;
$$;
