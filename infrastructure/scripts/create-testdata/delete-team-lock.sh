#!/bin/bash
set -eu
set -o pipefail
set -x

# shellcheck source=ports.sh
source "$(dirname "$0")/ports.sh"
env=development
team=${1}
lockId=${2}

url="${URL}${FRONTEND_PORT}/api/environments/${env}/lock/team/${team}/${lockId}"

curl -X DELETE "$url" -H 'Content-Type: application/json'

echo
