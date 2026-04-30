#!/bin/bash
set -eu
set -o pipefail
set -x

FRONTEND_PORT=${KUBERPULT_PORT_FRONTEND_HTTP:-8081}
env=staging
lockId=lockIdTest${RANDOM}
url="http://localhost:${FRONTEND_PORT}/environments/${env}/locks/${lockId}"

curl -X PUT "$url" -d '{"message": "test env lock"}' -H 'Content-Type: application/json'

echo



