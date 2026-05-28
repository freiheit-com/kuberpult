-- The argo_app column holds EITHER a plain kuberpult app name OR a bracket name.
-- is_bracket makes each row self-describe which kind it is, so the rollout-service
-- knows whether the Argo Application <env>-<argo_app> is an individual app or a bracket.
CREATE TABLE IF NOT EXISTS rollout_should_undeploy_cascade (
  created timestamp NOT NULL DEFAULT NOW(),
  argo_app VARCHAR NOT NULL,
  env VARCHAR NOT NULL,
  attempts INTEGER NOT NULL DEFAULT 0,
  not_before_transformer_esl_id BIGINT NOT NULL,
  is_bracket BOOLEAN NOT NULL DEFAULT false,
  PRIMARY KEY (argo_app, env)
);
