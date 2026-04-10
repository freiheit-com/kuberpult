#!/bin/bash
set -eu
set -o pipefail
#set -x

# for testing the trace origin ID
clientUUID="12345678-1234-1234-1234-123456789012"

env=${1:-fakeprod-ca}
if test "$#" -eq 2; then
  url="http://localhost:8081/api/environments/${env}/releasetrain?""$2"
else
  url="http://localhost:8081/api/environments/${env}/releasetrain"
fi


curl -X PUT -H "client-uuid:${clientUUID}" "$url"
echo



