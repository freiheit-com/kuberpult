#!/bin/bash
set -eu
set -o pipefail
set -x

FRONTEND_PORT=${KUBERPULT_PORT_FRONTEND_HTTP:-8081}
env=development
team=${1}
lockId=${2}

url="http://localhost:${FRONTEND_PORT}/api/environments/${env}/lock/team/${team}/${lockId}"

curl -X DELETE "$url" -H 'Content-Type: application/json'

echo
