#!/bin/bash
set -ueo pipefail
script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd -P)"
source "${script_dir}"/container.inc.sh

main_branch="main"
current_branch="$(git rev-parse --abbrev-ref HEAD)"
# When called locally with Makefile, the default parameters are used
base="${1:-$(git merge-base "${main_branch}" "${current_branch}")}"
head="${2:-$(git rev-parse HEAD)}"

git diff --diff-filter=ACMRDT --name-only "$base" "$head" | docker run -i -v "$(pwd)":/repo "${BUILDER_IMAGE}" pr
