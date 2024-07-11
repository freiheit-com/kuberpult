ALTER TABLE commit_events
    ADD COLUMN IF NOT EXISTS transformerEslId INTEGER
    CONSTRAINT fk_commit_events_transformer_id
        REFERENCES event_sourcing_light(eslId) default 0;