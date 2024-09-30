#!/bin/bash
set -eu
set -o pipefail

BASE_ENV_NAME=${1:-load-testing}
NUMBER_ENVS=${2:-50}
BASE_APP_NAME=${3}
NUMBER_RELEASES=${4:-100}

#/bin/bash ./create-environments.sh
#/bin/bash ./create-load-env-data.sh "$BASE_ENV_NAME" "$NUMBER_ENVS"
/bin/bash ./create-load-releases.sh "$BASE_APP_NAME" "sre-team" "$BASE_ENV_NAME" "$NUMBER_ENVS" "$NUMBER_RELEASES"
#/bin/bash ./run-releasetrain.sh

#for (( env_n=1; env_n<=NUMBER_ENVS; env_n++ ))
#    do
#    time /bin/bash ./run-releasetrain.sh $BASE_ENV_NAME-$env_n
#done

