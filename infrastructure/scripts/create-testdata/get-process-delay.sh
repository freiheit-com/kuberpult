#!/bin/bash
set -eu
set -o pipefail

# usage:
# ./get-process-delay.sh


FRONTEND_PORT=8081 # see docker-compose.yml

curl  -f -X GET  \
    "http://localhost:${FRONTEND_PORT}/api/process-delay/" | jq .
