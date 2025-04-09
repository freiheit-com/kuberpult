#!/bin/bash

# This script creates a few releases, while in parallel we call undeploy.
# This is useful to test the lock feature (db.lockType).
# For repeated runs, adapt the "MIN" and "MAX" variables below.
# To check if the test is successful or not:
# select applications from environments where name='development'; select appname from releases where appname='echo-???'
# The test is successful if EACH application either appear in both tables, or in neither.

cd "$(dirname "$0")" || exit 1

MIN=120
MAX=125

./create-environments.sh

for v in $(seq 1 1); do
  # Setup: ensure the app exists:
  for appId in $(seq -w $MIN $MAX ); do
    RELEASE_VERSION=99999 ~/projects/fdc/kuberpult/infrastructure/scripts/create-testdata/create-release.sh "echo-${appId}";
  done

  echo "Done with setup, starting test..."

# Test:
  for appId in $(seq -w $MIN $MAX ); do
    app="echo-${appId}"
    RELEASE_VERSION=$v ~/projects/fdc/kuberpult/infrastructure/scripts/create-testdata/create-release.sh "${app}" &
    echo '{
  "actions": [
    {
      "prepare_undeploy": {
        "application": "'${app}'"
      }
    },
    {
      "undeploy": {
        "application": "'${app}'"
      }
    }
  ]
}' | \
      evans --header author-name=YXV0aG9y --header author-email=YXV0aG9yQGF1dGhvcg== --host localhost --port 8443 -r cli call api.v1.BatchService.ProcessBatch &
  done
done

echo "waiting for all calls to finish..."
wait
echo "ALL DONE."
