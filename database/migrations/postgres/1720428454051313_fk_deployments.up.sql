DO $$ BEGIN IF NOT EXISTS (
    SELECT 'fk_deployments_transformer_id'
    FROM information_schema.table_constraints
    WHERE table_name = 'deployments'
        AND constraint_name = 'fk_deployments_transformer_id'
) THEN
ALTER TABLE deployments
ADD COLUMN IF NOT EXISTS transformerEslId INTEGER CONSTRAINT fk_deployments_transformer_id REFERENCES event_sourcing_light(eslId) default 0;
end if;
END $$;