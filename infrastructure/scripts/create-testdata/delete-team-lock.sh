#!/bin/bash
set -eu
set -o pipefail
set -x

env=development
team=${1}
lockId=${2}

url="http://localhost:8081/api/environments/${env}/lock/team/${team}/${lockId}"

curl -X DELETE "$url" -H 'Content-Type: application/json'

echo
