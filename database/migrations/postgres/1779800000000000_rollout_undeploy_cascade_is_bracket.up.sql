-- The argo_app column holds EITHER a plain kuberpult app name OR a bracket name.
-- is_bracket makes each row self-describe which kind it is, so the rollout-service
-- knows whether the Argo Application <env>-<argo_app> is an individual app or a bracket.
ALTER TABLE rollout_should_undeploy_cascade
    ADD COLUMN IF NOT EXISTS is_bracket BOOLEAN NOT NULL DEFAULT false;
