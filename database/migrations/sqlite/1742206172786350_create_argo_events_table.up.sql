CREATE TABLE IF NOT EXISTS argo_cd_events(
  created timestamp,
  app VARCHAR NOT NULL,
  env varchar NOT NULL,
  json varchar NOT NULL,
  discarded boolean default false,
  PRIMARY KEY(app, env)
);