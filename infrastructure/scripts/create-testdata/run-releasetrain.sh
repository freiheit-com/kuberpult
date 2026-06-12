#!/bin/bash
set -eu
set -o pipefail
#set -x

# for testing the trace origin ID
clientUUID="12345678-1234-1234-1234-123456789012"

# shellcheck source=ports.sh
source "$(dirname "$0")/ports.sh"
env=${1:-fakeprod-ca}
if test "$#" -eq 2; then
  url="${URL}:${FRONTEND_PORT}/api/environments/${env}/releasetrain?""$2"
else
  url="${URL}:${FRONTEND_PORT}/api/environments/${env}/releasetrain"
fi


curl -X PUT -H "client-uuid:${clientUUID}" "$url"
echo



