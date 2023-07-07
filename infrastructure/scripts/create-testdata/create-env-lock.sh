#!/bin/bash
set -eu
set -o pipefail
set -x

env=staging
lockId=lockIdTest${RANDOM}
url="http://localhost:8081/environments/${env}/locks/${lockId}"

curl -X PUT "$url" -d '{"message": "test env lock"}' -H 'Content-Type: application/json'

echo



