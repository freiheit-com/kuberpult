#!/bin/bash
set -eu
set -o pipefail
set -x

env=development
lockId=test${RANDOM}
app=${1}
url="http://localhost:8081/environments/${env}/applications/${app}/locks/${lockId}"

curl -X PUT "$url" -d '{"message": "test app lock"}' -H 'Content-Type: application/json'

echo



