#!/bin/bash
set -eu
set -o pipefail

# usage:
# ./get-process-delay.sh


# shellcheck source=ports.sh
source "$(dirname "$0")/ports.sh"

curl  -f -X GET  \
    "http://localhost:${FRONTEND_PORT}/api/process-delay/" | jq .
