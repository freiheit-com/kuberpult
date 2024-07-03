ALTER TABLE commit_events
ADD CONSTRAINT fk_commit_events_transformer_id FOREIGN KEY (transformerEslId)
REFERENCES event_sourcing_light (eslId);