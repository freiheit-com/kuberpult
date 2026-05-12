#!/bin/bash
# Usage: ./manifest-setup.sh [START_VERSION [NUM_VERSIONS [APP_NAME]]]
#
# Creates environments, releases, and runs a release train for local development.
# Ports are read automatically from docker-compose.yml via ports.sh.

set -eu
set -o pipefail

SCRIPT_DIR="$(dirname "$0")"

START_VERSION="${1:-100}"
NUM_VERSIONS="${2:-7}"

"${SCRIPT_DIR}/create-environments.sh" || echo "Warning: create-environments.sh failed, continuing anyway" >&2

APP_NAME="${3:-test-1}"
# Take first character of app name as Argo bracket prefix
ARGO_BRACKET=${APP_NAME:0:1}
echo "bracket: ${ARGO_BRACKET}"
echo "app:     ${APP_NAME}"

TEAM="sre"

for v in $(seq 1 "$NUM_VERSIONS"); do
    echo "$v"
    REVISION=$v RELEASE_VERSION=$((START_VERSION + v)) \
        "${SCRIPT_DIR}/create-release.sh" "${APP_NAME}" "${TEAM}" "" "${ARGO_BRACKET}"
done

"${SCRIPT_DIR}/run-releasetrain.sh" staging "team=${TEAM}"

# One extra release so not all apps are in the same state
REVISION=6 RELEASE_VERSION=$((START_VERSION + 7)) \
    "${SCRIPT_DIR}/create-release.sh" "${APP_NAME}" "${TEAM}" "" "${ARGO_BRACKET}"
