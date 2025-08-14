-- This index was created for the manifest-export to quickly get all deployments of an env
-- see DBSelectAppsWithDeploymentInEnvAtTimestamp
CREATE INDEX IF NOT EXISTS idx_deployments_history_app_env_created
ON deployments_history (appName, envName, created DESC);
