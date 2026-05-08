#!/bin/bash
set -eu
set -o pipefail
# usage
# ./create-environments.sh [path/to/envs]
# Note that this just creates files, it doesn't push in git
# See ./Readme.md for how to generate a signature ("$env".yaml.sig)

cd "$(dirname "$0")"

# shellcheck source=ports.sh
source "$(dirname "$0")/ports.sh"
env=${1}
curl  -f -X DELETE  \
    --form signature=@"$env".yaml.sig \
    http://localhost:"${FRONTEND_PORT}"/api/environments/"${env}" -v

echo # curl sometimes does not print a trailing \n
