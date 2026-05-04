#!/bin/bash
set -eu
set -o pipefail
# usage
# ./create-environments.sh [path/to/envs]
# Note that this just creates files, it doesn't push in git

cd "$(dirname "$0")"

# shellcheck source=ports.sh
source "$(dirname "$0")/ports.sh"
env=${1}

curl  -f -X DELETE  \
    http://localhost:"${FRONTEND_PORT}"/api/environments/"${env}"

echo # curl sometimes does not print a trailing \n
