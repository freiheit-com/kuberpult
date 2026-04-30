#!/bin/bash
set -eu
set -o pipefail
set -x

# shellcheck source=ports.sh
source "$(dirname "$0")/ports.sh"
env=staging

lockId=${1}
url="http://localhost:${FRONTEND_PORT}/environments/${env}/locks/${lockId}"

curl -X DELETE "$url" -H 'Content-Type: application/json'

echo



