#!/bin/bash
set -eu
set -o pipefail
set -x

envGroup=prod

lockId=${1}
url="http://localhost:8081/environment-groups/${envGroup}/locks/${lockId}"

curl -X DELETE "$url" -H 'Content-Type: application/json'

echo



