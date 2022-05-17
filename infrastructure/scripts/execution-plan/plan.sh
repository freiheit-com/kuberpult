#!/bin/bash
set -ueo pipefail

main_branch="main"
current_branch="$(git rev-parse --abbrev-ref HEAD)"
# When called locally with Makefile, the default parameters are used
base="${1:-$(git merge-base "${main_branch}" "${current_branch}")}"
head="${2:-$(git rev-parse HEAD)}"

SCRIPT_DIR=$( cd -- "$( dirname -- "${BASH_SOURCE[0]}" )" &> /dev/null && pwd )
"$SCRIPT_DIR"/get-changed-files-pr.sh "$base" "$head" | "$SCRIPT_DIR"/execution-plan.sh
