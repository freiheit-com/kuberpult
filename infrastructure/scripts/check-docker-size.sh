#!/usr/bin/env bash
set -euo pipefail

if (( $# < 3 )); then
    echo "usage: $0 <docker-image-without-tag> <SIZE_THRESHOLD_MB> <PRODUCT_NAME>" >&2
    exit 2
fi

DOCKER_IMAGE=$1
SIZE_THRESHOLD_MB=$2
PRODUCT_NAME=$3
# Measure the docker image size is rather unprecise, so we allow a discrepancy in percent:
DISCREPANCY_PERCENT=10

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
ACTUAL_SIZE_MB=$(awk -v val="${ACTUAL_SIZE}" 'BEGIN { printf "%.0f\n", val / 1000000 }');
THRESHOLD_PLUS_BUFFER_MB=$(awk -v val="${SIZE_THRESHOLD_MB}" -v discrepancy="${DISCREPANCY_PERCENT}" 'BEGIN { printf "%.0f\n", val * (1+discrepancy/100) }');
THRESHOLD_MINUS_BUFFER_MB=$(awk -v val="${SIZE_THRESHOLD_MB}" -v discrepancy="${DISCREPANCY_PERCENT}" 'BEGIN { printf "%.0f\n", val * (1-discrepancy/100) }');

if [ "$(awk "BEGIN { print ($ACTUAL_SIZE_MB > $THRESHOLD_PLUS_BUFFER_MB) ? 1 : 0 }")" = 1 ]; then
    echo "Image too large: Your docker image has ${ACTUAL_SIZE_MB}MB. The configured threshold is ${SIZE_THRESHOLD_MB}MB. Even with the ${DISCREPANCY_PERCENT}% discrepancy buffer, that is ${THRESHOLD_PLUS_BUFFER_MB}MB.";
    exit 1;
else
    echo "actual: ${ACTUAL_SIZE_MB}"
    echo "threshold-: ${THRESHOLD_MINUS_BUFFER_MB}"
    if [ "$(awk "BEGIN { print ($ACTUAL_SIZE_MB < $THRESHOLD_MINUS_BUFFER_MB)? 1 : 0 }")" = 1 ]; then
        echo "Image too small: Your docker image has ${ACTUAL_SIZE_MB}MB. The configured threshold is ${SIZE_THRESHOLD_MB}MB. Even with the ${DISCREPANCY_PERCENT}% discrepancy buffer, that is ${THRESHOLD_MINUS_BUFFER_MB}MB.";
        exit 1;
    fi

    echo "Docker image is sufficiently small: ${ACTUAL_SIZE_MB}MB<=${SIZE_THRESHOLD_MB}MB in \"$PRODUCT_NAME\" image ${DOCKER_IMAGE}";
fi
