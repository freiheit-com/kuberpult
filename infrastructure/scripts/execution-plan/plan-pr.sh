#!/bin/bash
set -ueo pipefail
script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd -P)"
cd "${script_dir}"
# shellcheck disable=SC1091
source "${script_dir}"/container.inc.sh

main_branch="main"
current_branch="$(git rev-parse --abbrev-ref HEAD)"
# When called locally with Makefile, the default parameters are used
base="${2:-$(git merge-base "${main_branch}" "${current_branch}")}"
head="${3:-$(git rev-parse HEAD)}"

git diff --diff-filter=ACMRDT --name-only "$base" "$head" | ./create-matrix.sh "${1}"
