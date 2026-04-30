#!/bin/bash
set -eu
set -o pipefail

# usage:
# ./get-process-delay.sh


FRONTEND_PORT=${KUBERPULT_PORT_FRONTEND_HTTP:-8081} # see docker-compose.yml

curl  -f -X GET  \
    "http://localhost:${FRONTEND_PORT}/api/process-delay/" | jq .
