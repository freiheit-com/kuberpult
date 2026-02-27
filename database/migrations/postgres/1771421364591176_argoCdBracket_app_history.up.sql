-- this is identical to the previous migration, except it's for the table "apps_history"

ALTER TABLE IF EXISTS apps_history ADD COLUMN IF NOT EXISTS argoBracket varchar(63) ;

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1
        FROM pg_constraint
        WHERE conname = 'apps_history_argo_cd_bracket_not_empty'
    ) THEN
ALTER TABLE apps_history
    ADD CONSTRAINT apps_history_argo_cd_bracket_not_empty CHECK (argoBracket <> '');
END IF;
END;
$$;

COMMENT ON CONSTRAINT apps_history_argo_cd_bracket_not_empty ON apps_history
IS 'argo brackets cannot be empty, use NULL instead';
