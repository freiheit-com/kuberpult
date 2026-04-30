#!/bin/bash
set -eu
set -o pipefail
set -x

FRONTEND_PORT=${KUBERPULT_PORT_FRONTEND_HTTP:-8081}
envGroup=prod

lockId=${1}
url="http://localhost:${FRONTEND_PORT}/environment-groups/${envGroup}/locks/${lockId}"

curl -X DELETE "$url" -H 'Content-Type: application/json'

echo



