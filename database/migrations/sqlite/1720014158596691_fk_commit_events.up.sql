ALTER TABLE commit_events
    ADD COLUMN transformerEslId INTEGER
        REFERENCES event_sourcing_light(eslId) default 0;