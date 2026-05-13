#!/bin/bash
set -eu
set -o pipefail
# usage
# ./create-environments.sh [path/to/envs]

# shellcheck source=ports.sh
source "$(dirname "$0")/ports.sh"
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
           ${KUBERPULT_BASE_URL}/environments/"${env}"
  else
    curl  -w "%{http_code}\n" -X POST -H "multipart/form-data" \
          --form-string "config=${DATA}" \
           ${KUBERPULT_BASE_URL}/api/environments/"${env}"
  fi
  echo # curl sometimes does not print a trailing \n
done
