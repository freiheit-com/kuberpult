#!/bin/bash
set -ueo pipefail

SCRIPT_DIR=$( cd -- "$( dirname -- "${BASH_SOURCE[0]}" )" &> /dev/null && pwd )
"$SCRIPT_DIR"/get-changed-files.sh ${1:-"main"} ${2:-"HEAD"} | "$SCRIPT_DIR"/execution-plan.sh
