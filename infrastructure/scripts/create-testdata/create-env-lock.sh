#!/bin/bash
set -eu
set -o pipefail
set -x

# shellcheck source=ports.sh
source "$(dirname "$0")/ports.sh"
env=staging
lockId=lockIdTest${RANDOM}
url="${URL}${FRONTEND_PORT}/environments/${env}/locks/${lockId}"

curl -X PUT "$url" -d '{"message": "test env lock"}' -H 'Content-Type: application/json'

echo



