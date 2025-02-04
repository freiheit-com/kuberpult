DO $$ BEGIN IF NOT EXISTS (
    SELECT column_name
    FROM information_schema.columns
    WHERE table_name='deployments' and column_name='transformerEslversion'
) THEN
ALTER TABLE deployments
ADD COLUMN IF NOT EXISTS transformerEslId INTEGER default 0;
end if;
END $$;