#!/bin/bash
# Source this file to get the actual host ports.
# Reads the resolved configuration from docker-compose.yml (honoring .env overrides)
# when docker and jq are available; otherwise falls back to the default ports.

FRONTEND_PORT="${FRONTEND_PORT:-}"
CD_GRPC_PORT="${CD_GRPC_PORT:-}"

_REPO_ROOT=$(git -C "$(dirname "${BASH_SOURCE[0]}")" rev-parse --show-toplevel 2>/dev/null || true)

if [ -f "${_REPO_ROOT}/.env.local" ]; then
    set -a
    # shellcheck source=/dev/null
    . "${_REPO_ROOT}/.env.local"
    set +a
fi

if [ -n "$_REPO_ROOT" ] && command -v docker &>/dev/null && command -v jq &>/dev/null; then
    _DC_JSON=$(docker compose -f "${_REPO_ROOT}/docker-compose.yml" config --format json 2>/dev/null || true)
    if [ -n "$_DC_JSON" ]; then
        FRONTEND_PORT=$(printf '%s' "$_DC_JSON" | jq -r '.services["frontend-service"].ports[] | select(.target == 8081) | .published // empty' 2>/dev/null || true)
        CD_GRPC_PORT=$(printf '%s' "$_DC_JSON" | jq -r '.services["cd-service"].ports[] | select(.target == 8443) | .published // empty' 2>/dev/null || true)
    fi
fi

if [ -z "$FRONTEND_PORT" ]; then
    echo "Warning: could not determine FRONTEND_PORT from docker-compose; using default 8081" >&2
fi
if [ -z "$CD_GRPC_PORT" ]; then
    echo "Warning: could not determine CD_GRPC_PORT from docker-compose; using default 8443" >&2
fi

FRONTEND_PORT=${FRONTEND_PORT:-8081}
CD_GRPC_PORT=${CD_GRPC_PORT:-8443}

export FRONTEND_PORT CD_GRPC_PORT
unset _REPO_ROOT _DC_JSON
