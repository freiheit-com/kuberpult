SELECT *
FROM environments
WHERE EXISTS (
    SELECT 1
    FROM json_array_elements_text(environments.applications::json) AS env_app(name)
    WHERE env_app.name NOT IN (SELECT appname FROM apps)
);

SELECT *
FROM releases
WHERE EXISTS (
  SELECT 1
  FROM jsonb_object_keys(manifests::jsonb -> 'Manifests') AS manifest_env
  WHERE NOT EXISTS (
    SELECT 1
    FROM json_array_elements_text(environments::json) AS env
    WHERE env = manifest_env
  )
)
OR EXISTS (
  SELECT 1
  FROM json_array_elements_text(environments::json) AS env
  WHERE NOT EXISTS (
    SELECT 1
    FROM jsonb_object_keys(manifests::jsonb -> 'Manifests') AS manifest_env
    WHERE manifest_env = env
  )
);

SELECT *
FROM deployments 
WHERE envname NOT IN (
  SELECT DISTINCT json_array_elements_text(releases.environments::json) FROM releases
);

