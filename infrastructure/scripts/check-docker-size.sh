#!/usr/bin/env bash
set -euo pipefail

if (( $# < 3 )); then
    echo "usage: $0 <docker-image> <SIZE_THRESHOLD_MB> <PRODUCT_NAME>" >&2
    exit 2
fi

DOCKER_IMAGE=$1
SIZE_THRESHOLD_MB=$2
# Measure the docker image size is rather unprecise, so we allow a discrepancy in percent (both above and below):
DISCREPANCY_PERCENT=10
SCRIPT_DIR=$(dirname "$0")

# fail if the threshold is not a number with 1-2 digits before the point and up to 5 after the point:
re='^[0-9]{1,3}(.[0-9]{1,5})?$'
if ! [[ ${SIZE_THRESHOLD_MB} =~ $re ]] ; then
   echo "error: Not a number: ${SIZE_THRESHOLD_MB}" >&2; exit 1
fi

# skip coverage test if threshold is set to 0.0 or below
if [ "$(awk "BEGIN { print ($SIZE_THRESHOLD_MB < 0.0)? 1 : 0 }")" = 1 ]; then
    echo "Skipping docker size check";
    exit 0;
fi

ACTUAL_SIZE=$(docker image inspect --format='{{.Size}}' "${DOCKER_IMAGE}")

echo "${ACTUAL_SIZE} ${SIZE_THRESHOLD_MB} ${DISCREPANCY_PERCENT}"|"${SCRIPT_DIR}/check-docker-size.awk"
