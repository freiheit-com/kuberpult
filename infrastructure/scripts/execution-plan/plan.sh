#!/bin/sh
set -u
set -e
set -o pipefail

SCRIPT_DIR=$( cd -- "$( dirname -- "${BASH_SOURCE[0]}" )" &> /dev/null && pwd )
"$SCRIPT_DIR"/get-changed-files.sh | "$SCRIPT_DIR"/execution-plan.sh
