CREATE TABLE IF NOT EXISTS manifest_locks_history
(
    lockid      SERIAL,
    recorded_at TIMESTAMP,
    app         VARCHAR,
    env         VARCHAR,
    metadata    VARCHAR,
    active      BOOLEAN,
    event_type  VARCHAR,
    PRIMARY KEY (lockid)
);

CREATE INDEX IF NOT EXISTS manifest_locks_history_active_idx
    ON manifest_locks_history (active);

CREATE INDEX IF NOT EXISTS manifest_locks_history_app_env_active_idx
    ON manifest_locks_history (app, env, active);
