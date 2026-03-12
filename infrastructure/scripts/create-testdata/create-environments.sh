#!/bin/bash
set -eu
set -o pipefail
# usage
# ./create-environments.sh [path/to/envs]
# Note that this just creates files, it doesn't push in git

FRONTEND_PORT=8081 # see docker-compose.yml
cd "$(dirname "$0")"
testData=${1:-"./testdata_template/environments"}
useOldApi=false

for filename in "$testData"/*; do
  configFile="$filename"/config.json
  env=$(basename -- "$filename")
  env=$(echo "$env" | awk '{print tolower($0)}')
  echo Writing "$env"...
  DATA=$(cat "$configFile")
  echo "useOldApi=$useOldApi"
  if $useOldApi; then
    curl  -f -X POST -H "multipart/form-data" \
          --form-string "config=${DATA}" \
           http://localhost:${FRONTEND_PORT}/environments/"${env}"
  else
    curl -X POST -H "multipart/form-data" \
          --form-string "config=${DATA}" \
           http://localhost:${FRONTEND_PORT}/api/environments/"${env}"
  fi
done

echo # curl sometimes does not print a trailing \n
