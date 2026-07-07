#!/bin/bash
# Usage: ./manifest-setup.sh [START_VERSION [NUM_VERSIONS [TRAIN_INTERVAL [APP_NAME [TEAM]]]]]
#
# Creates environments, releases, git tags, and runs release trains for local development.
# A release train (carrying a git tag) is triggered after every TRAIN_INTERVAL releases.
# Ports are read automatically from docker-compose.yml via ports.sh.

set -eu
set -o pipefail

SCRIPT_DIR="$(dirname "$0")"

START_VERSION="${1:-100}"
NUM_VERSIONS="${2:-7}"
TRAIN_INTERVAL="${3:-3}"
APP_NAME="${4:-test-1}"
TEAM="${5:-sreteam}"

"${SCRIPT_DIR}/create-environments.sh" || echo "Warning: create-environments.sh failed, continuing anyway" >&2

# Take first character of app name as Argo bracket prefix
ARGO_BRACKET=${APP_NAME:0:1}
echo "bracket: ${ARGO_BRACKET}"
echo "app:     ${APP_NAME}"
echo "team:    ${TEAM}"

# Each release is auto-deployed to development (upstream.latest). A release train
# promotes development's current version to staging (staging.upstream =
# development) and, because that produces a new manifest commit, pushes a git
# tag. We only run the train after every TRAIN_INTERVAL releases, so tags are
# created periodically rather than once per release.
train_count=0
for v in $(seq 1 "$NUM_VERSIONS"); do
    echo "release ${v}"
    REVISION=$v RELEASE_VERSION=$((START_VERSION + v)) \
        "${SCRIPT_DIR}/create-release-allparams.sh" "${APP_NAME}" "${TEAM}" "${ARGO_BRACKET}"
    if (( v % TRAIN_INTERVAL == 0 )); then
        train_count=$((train_count + 1))
        echo "triggering release train with git tag setup-tag-${APP_NAME}-${train_count}"
        "${SCRIPT_DIR}/run-releasetrain.sh" staging "team=${TEAM}&gitTag=setup-tag-${APP_NAME}-${train_count}"
    fi
    NEXT_VERSION=$((START_VERSION + v + 1))
done

# Promote and tag any releases created after the last interval-triggered train.
if (( NUM_VERSIONS % TRAIN_INTERVAL != 0 )); then
    train_count=$((train_count + 1))
    echo "triggering final release train with git tag setup-tag-${APP_NAME}-${train_count}"
    "${SCRIPT_DIR}/run-releasetrain.sh" staging "team=${TEAM}&gitTag=setup-tag-${APP_NAME}-${train_count}"
fi

# One extra release, left un-promoted, so not all apps are in the same state
REVISION=6 RELEASE_VERSION=$NEXT_VERSION \
    "${SCRIPT_DIR}/create-release-allparams.sh" "${APP_NAME}" "${TEAM}" "${ARGO_BRACKET}"
