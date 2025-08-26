#!/usr/bin/env bash
set -euo pipefail

if (( $# < 3 )); then
    echo "usage: $0 <docker-image> <SIZE_THRESHOLD_MB> <PRODUCT_NAME>" >&2
    exit 2
fi

DOCKER_IMAGE=$1
SIZE_THRESHOLD_MB=$2
PRODUCT_NAME=$3
DISCREPANCY_MB=10

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

ACTUAL_SIZE=$(docker inspect -f '{{ .Size }}' "${DOCKER_IMAGE}")
ACTUAL_SIZE_MB=$(awk -v val="${ACTUAL_SIZE}" 'BEGIN { printf "%.0f\n", val / 1000000 }');

if [ "$(awk "BEGIN { print ($ACTUAL_SIZE_MB > $SIZE_THRESHOLD_MB) ? 1 : 0 }")" = 1 ]; then
    echo "Docker size too high ${ACTUAL_SIZE_MB}MB>${SIZE_THRESHOLD_MB}MB in $PRODUCT_NAME image ${DOCKER_IMAGE}";
    echo "Running docker history:"
    docker history "${DOCKER_IMAGE}"
    exit 1;
else
    if [ "$(awk "BEGIN { print ($ACTUAL_SIZE_MB < $SIZE_THRESHOLD_MB - $DISCREPANCY_MB)? 1 : 0 }")" = 1 ]; then
        echo "The current docker image size ${ACTUAL_SIZE_MB}MB is more than ${DISCREPANCY_MB}MB smaller than the threshold of ${SIZE_THRESHOLD_MB}MB. Please adjust the threshold of ${PRODUCT_NAME} accordingly!"
        echo "Running docker history:"
        docker history "${DOCKER_IMAGE}"
        exit 1;
    fi

    echo "Docker image is sufficiently small: ${ACTUAL_SIZE_MB}MB<=${SIZE_THRESHOLD_MB}MB in $PRODUCT_NAME image ${DOCKER_IMAGE}";
fi
