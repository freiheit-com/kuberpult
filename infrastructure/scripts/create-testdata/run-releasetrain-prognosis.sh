#!/bin/bash
set -eu
set -o pipefail
#set -x

env=${1:-fakeprod-ca}
url="http://localhost:8081/api/environments/${env}/releasetrain/prognosis"


curl -X GET "$url"

echo
