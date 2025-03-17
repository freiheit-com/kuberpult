CREATE TABLE IF NOT EXISTS argo_cd_events(
  created timestamp,
  app VARCHAR NOT NULL,
  env varchar NOT NULL,
  json varchar NOT NULL,
  PRIMARY KEY(app, env)
);