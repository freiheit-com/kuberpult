
-- checks if an app's state in the DB is consistent:
-- apps table, releases table, and environments table

WITH env_has_app AS (
    SELECT EXISTS (
        SELECT 1
        FROM environments
        WHERE name = 'development'
          AND applications::jsonb @> json_build_array(:'app'::text)::jsonb
    ) AS result
),
     app_has_release AS (
         SELECT EXISTS (
             SELECT 1
             FROM releases
             WHERE appname = :'app'
             ORDER BY releaseVersion DESC
         ) AS result
     ),
    app_was_created AS (
        SELECT EXISTS (
            select appname, statechange
            from apps
            WHERE appname = :'app'
            AND statechange='AppStateChangeCreate'
       ) AS result
)
SELECT
    (SELECT result FROM app_was_created) AS "App created in apps table",
    (SELECT result FROM env_has_app) AS "Environment has App",
    (SELECT result FROM app_has_release) AS "App has Release(s)",
    ((SELECT result FROM env_has_app) = (SELECT result FROM app_has_release)) AS "All consistent";