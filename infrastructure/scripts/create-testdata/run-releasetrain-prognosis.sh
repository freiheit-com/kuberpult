#!/bin/bash
set -eu
set -o pipefail
#set -x

# shellcheck source=ports.sh
source "$(dirname "$0")/ports.sh"
env=${1:-fakeprod-ca}
url="http://localhost:${FRONTEND_PORT}/api/environments/${env}/releasetrain/prognosis"


curl -X GET "$url"

echo
