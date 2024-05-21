#!/bin/bash
set -eu
set -o pipefail
set -x


team=${1}
env=${2:-development}
lockId=test${RANDOM}

url="http://localhost:8081/api/environments/${env}/lock/team/${team}/${lockId}"

curl -X PUT "$url" -d '{"message": "test team lock"}' -H 'Content-Type: application/json'

echo



