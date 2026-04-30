#!/bin/bash
set -eu
set -o pipefail
set -x

# shellcheck source=ports.sh
source "$(dirname "$0")/ports.sh"
envGroup=prod

lockId=${1}
url="http://localhost:${FRONTEND_PORT}/environment-groups/${envGroup}/locks/${lockId}"

curl -X DELETE "$url" -H 'Content-Type: application/json'

echo



