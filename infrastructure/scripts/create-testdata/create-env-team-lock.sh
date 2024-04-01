#!/bin/bash
set -eu
set -o pipefail
set -x

env=staging
lockId=test${RANDOM}
team=${1}
url="http://localhost:8081/api/environments/${env}/lock/team/${team}/${lockId}"

curl -X PUT "$url" -d '{"message": "test app lock"}' -H 'Content-Type: application/json'

echo



