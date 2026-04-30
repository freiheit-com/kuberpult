#!/bin/bash
set -eu
set -o pipefail
#set -x

# for testing the trace origin ID
clientUUID="12345678-1234-1234-1234-123456789012"

FRONTEND_PORT=${KUBERPULT_PORT_FRONTEND_HTTP:-8081}
env=${1:-fakeprod-ca}
if test "$#" -eq 2; then
  url="http://localhost:${FRONTEND_PORT}/api/environments/${env}/releasetrain?""$2"
else
  url="http://localhost:${FRONTEND_PORT}/api/environments/${env}/releasetrain"
fi


curl -X PUT -H "client-uuid:${clientUUID}" "$url"
echo



