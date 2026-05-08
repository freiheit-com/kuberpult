#!/bin/bash
set -eu
set -o pipefail
set -x


# shellcheck source=ports.sh
source "$(dirname "$0")/ports.sh"
team=${1}
env=${2:-development}
lockId=test${RANDOM}

url="http://localhost:${FRONTEND_PORT}/api/environments/${env}/lock/team/${team}/${lockId}"

curl -X PUT "$url" -d '{"message": "test team lock"}' -H 'Content-Type: application/json'

echo



