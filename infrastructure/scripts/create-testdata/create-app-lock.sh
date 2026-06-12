#!/bin/bash
set -eu
set -o pipefail
set -x

# shellcheck source=ports.sh
source "$(dirname "$0")/ports.sh"
env=staging
lockId=test${RANDOM}
app=${1}
url="${URL}:${FRONTEND_PORT}/environments/${env}/applications/${app}/locks/${lockId}"

curl -X PUT "$url" -d '{"message": "test app lock"}' -H 'Content-Type: application/json'

echo



