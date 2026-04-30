#!/bin/bash
# Source this file to get the actual host ports from docker-compose.yml.
# Reads the resolved configuration (honoring .env overrides) via docker compose.
# Requires: docker, jq

_REPO_ROOT=$(git -C "$(dirname "${BASH_SOURCE[0]}")" rev-parse --show-toplevel)
_DC_JSON=$(docker compose -f "${_REPO_ROOT}/docker-compose.yml" config --format json 2>/dev/null)

FRONTEND_PORT=$(printf '%s' "$_DC_JSON" | jq -r '.services["frontend-service"].ports[] | select(.target == 8081) | .published')
CD_GRPC_PORT=$(printf '%s' "$_DC_JSON" | jq -r '.services["cd-service"].ports[] | select(.target == 8443) | .published')

export FRONTEND_PORT CD_GRPC_PORT
unset _REPO_ROOT _DC_JSON
