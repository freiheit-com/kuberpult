#!/bin/bash

# create-many-releases.sh MIN MAX
# Where MIN is the first release number used, and MAX is the last. Make sure that MAX>=MIN and MIN>=0.
# This script creates a few releases, while in parallel we call undeploy.
# This is useful to test the lock feature (db.lockType).
# To check if the test is successful or not:
# select applications from environments where name='development'; select appname from releases where appname='echo-???'
# The test is successful if EACH application either appear in both tables, or in neither. See query.sql for the test.

cd "$(dirname "$0")" || exit 1

set -u

MIN=${1}
MAX=${2}

./create-environments.sh

# Setup: ensure the app exists:
for appId in $(seq -w "${MIN}" "${MAX}" ); do
  RELEASE_VERSION=99999 ./create-release.sh "echo-${appId}";
done

echo "Done with setup, starting test..."

# Test:
for appId in $(seq -w "${MIN}" "${MAX}" ); do
  app="echo-${appId}"
  RELEASE_VERSION=1 ./create-release.sh "${app}" &
  (
    echo '{
"actions": [
  {
    "prepare_undeploy": {
      "application": "'"${app}"'"
    }
  },
  {
    "undeploy": {
      "application": "'"${app}"'"
    }
  }
]
}' | \
    evans --header author-name=YXV0aG9y --header author-email=YXV0aG9yQGF1dGhvcg== --host localhost --port 8443 -r cli call api.v1.BatchService.ProcessBatch \
    && echo "app ${appId} deletion success" \
    || echo "app ${appId} deletion failed"
  ) &
done

echo "waiting for all calls to finish..."
wait
echo "ALL DONE."
