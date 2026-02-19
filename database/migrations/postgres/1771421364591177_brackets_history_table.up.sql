CREATE TABLE IF NOT EXISTS brackets_history
(
    esl_id SERIAL,
    created_at TIMESTAMP,
    all_brackets VARCHAR, -- all brackets as one JSON blob that is a map[bracket][]appName
    PRIMARY KEY(esl_id)
);

-- we always access this table by "created" column, so we index it:
CREATE INDEX IF NOT EXISTS brackets_history_created_idx on brackets_history (created_at);


-- Notes
-- 0) select apps_teams_history => Pairs(app,team) (combined with HasRelease)
---  0.1) make map out of pairs map[app]team
-- 1) SELECT bracket row
-- 2A) For each bracket:
--   2.1) extract apps from bracket row
--   2.2) get team from map in 0.1)
--   2.3) render bracket
-- 2B) if there are no brackets:
--   2.1) use apps from map

