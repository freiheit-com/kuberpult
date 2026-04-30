#!/bin/bash
set -eu
set -o pipefail
#set -x

FRONTEND_PORT=${KUBERPULT_PORT_FRONTEND_HTTP:-8081}
env=${1:-fakeprod-ca}
url="http://localhost:${FRONTEND_PORT}/api/environments/${env}/releasetrain/prognosis"


curl -X GET "$url"

echo
