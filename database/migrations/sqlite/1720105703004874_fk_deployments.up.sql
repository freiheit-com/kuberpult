ALTER TABLE deployments
    ADD COLUMN transformerEslId INTEGER
        REFERENCES event_sourcing_light(eslId) default 0;