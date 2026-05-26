CREATE TABLE IF NOT EXISTS rollout_should_undeploy_cascade (
  created timestamp NOT NULL DEFAULT NOW(),
  argo_app VARCHAR NOT NULL,
  env VARCHAR NOT NULL,
  attempts INTEGER NOT NULL DEFAULT 0,
  not_before_transformer_esl_id BIGINT NOT NULL,
  PRIMARY KEY (argo_app, env)
);
