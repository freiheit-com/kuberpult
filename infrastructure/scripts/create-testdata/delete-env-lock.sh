#!/bin/bash
set -eu
set -o pipefail
set -x

FRONTEND_PORT=${KUBERPULT_PORT_FRONTEND_HTTP:-8081}
env=staging

lockId=${1}
url="http://localhost:${FRONTEND_PORT}/environments/${env}/locks/${lockId}"

curl -X DELETE "$url" -H 'Content-Type: application/json'

echo



