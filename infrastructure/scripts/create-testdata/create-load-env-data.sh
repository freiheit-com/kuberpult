#!/bin/bash
set -eu
set -o pipefail

# Authenticate over IAP
BASE_ENV_NAME=${1:-loadTesting}
NUMBER_ENVS=${2:-50}

DATA="$(cat ./testdata_template/environments/loadTestingTemplate/config.json)"
FRONTEND_PORT=8081
for (( c=1; c<=NUMBER_ENVS; c++ ))
do
    curl -f -X POST \
      -H "multipart/form-data" --form-string "config=$DATA" \
      http://localhost:${FRONTEND_PORT}/environments/"$BASE_ENV_NAME-${c}" -v
    echo
done


