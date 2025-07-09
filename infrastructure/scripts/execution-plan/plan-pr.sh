#!/bin/bash
set -ueo pipefail
set -x
script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd -P)"
# shellcheck disable=SC1091
source "${script_dir}"/container.inc.sh

make -C infrastructure/tools/execplan exec-plan-compile

main_branch="main"
current_branch="$(git rev-parse --abbrev-ref HEAD)"
# When called locally with Makefile, the default parameters are used
base="${1:-$(git merge-base "${main_branch}" "${current_branch}")}"
head="${2:-$(git rev-parse HEAD)}"

git diff --diff-filter=ACMRDT --name-only "$base" "$head" #| infrastructure/tools/execplan/plan pr
