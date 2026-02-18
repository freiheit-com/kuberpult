ALTER TABLE IF EXISTS apps ADD COLUMN IF NOT EXISTS argoBracket varchar(63) ;

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1
        FROM pg_constraint
        WHERE conname = 'argo_cd_bracket_not_empty'
    ) THEN
ALTER TABLE apps
    ADD CONSTRAINT apps_argo_cd_bracket_not_empty CHECK (argoBracket <> '');
END IF;
END;
$$;

COMMENT ON CONSTRAINT apps_argo_cd_bracket_not_empty ON apps
IS 'argo brackets cannot be empty, use NULL instead';
