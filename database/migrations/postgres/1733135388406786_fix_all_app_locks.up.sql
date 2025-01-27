DO $$
BEGIN
    IF EXISTS (
        SELECT FROM information_schema.tables 
        WHERE table_name = 'all_app_locks'
    ) AND EXISTS (SELECT 1 
               FROM information_schema.columns 
               WHERE table_name = 'app_locks' 
                 AND column_name = 'eslversion') THEN
      EXECUTE 'WITH combinations AS (
        SELECT
          DISTINCT environment,
          appname
        FROM
          all_app_locks ),
      latest_app_locks_versions AS (
        SELECT MAX(eslversion) AS latest,
            envname,
            appname,
            lockID FROM app_locks GROUP BY envname, appname, lockId
      ),
      new_data AS (
        SELECT
          c.environment,
          c.appname,
          JSON_BUILD_OBJECT(''appLocks'', COALESCE(JSON_AGG(t.lockID) FILTER (WHERE t.lockID IS NOT NULL), ''[]''::json)) AS json
        FROM
          combinations c
        LEFT JOIN (SELECT al.eslversion, al.envname, al.appname, al.lockId FROM latest_app_locks_versions la
        JOIN app_locks al ON al.eslversion=la.latest AND al.envname=la.envname AND al.appname=la.appname AND al.lockId=la.lockId WHERE deleted=false) AS t
        ON
          c.environment = t.envname
          AND c.appname = t.appname
        GROUP BY
          c.environment,
          c.appname ),
      latest_versions AS (
        SELECT
          environment,
          appname,
          COALESCE(MAX(version), 0) AS max_version
        FROM
          all_app_locks
        GROUP BY
          environment,
          appname )
      INSERT INTO all_app_locks (version, created, environment, appname, json)
      SELECT max_version + 1, now(), lv.environment, lv.appname, t.json FROM new_data t LEFT JOIN latest_versions lv ON t.environment = lv.environment AND t.appname = lv.appname;';
    END IF;
END
$$;
INSERT INTO overview_cache (timestamp, json) VALUES (now(), '{}');