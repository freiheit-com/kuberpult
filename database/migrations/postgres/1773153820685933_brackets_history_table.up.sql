CREATE TABLE IF NOT EXISTS brackets_history
(
    esl_id SERIAL,
    created_at TIMESTAMP,
    all_brackets VARCHAR, -- all brackets as one JSON blob that is a map[bracket][]appName
    PRIMARY KEY(esl_id)
);

-- we always access this table by "created" column, so we index it:
CREATE INDEX IF NOT EXISTS brackets_history_created_idx on brackets_history (created_at);
