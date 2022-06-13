#!/bin/bash
set -ueo pipefail
SCRIPT_DIR="$( cd -- "$( dirname -- "${BASH_SOURCE[0]:-$0}"; )" &> /dev/null && pwd 2> /dev/null; )";

main_branch="main"
current_branch="$(git rev-parse --abbrev-ref HEAD)"
# When called locally with Makefile, the default parameters are used
base="${1:-$(git merge-base "${main_branch}" "${current_branch}")}"
head="${2:-$(git rev-parse HEAD)}"

git diff --diff-filter=ACMRT --name-only "$base" "$head" | docker run -i -v "$(pwd)":/repo $($SCRIPT_DIR/container.sh) build-pr
