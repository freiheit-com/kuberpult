#!/bin/bash
set -eu
set -o pipefail
set -x

env=staging

lockId=${1}
url="http://localhost:8081/environments/${env}/locks/${lockId}"

curl -X DELETE "$url" -H 'Content-Type: application/json'

echo



