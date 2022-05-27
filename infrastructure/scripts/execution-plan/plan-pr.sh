#!/bin/bash
set -ueo pipefail

main_branch="main"
current_branch="$(git rev-parse --abbrev-ref HEAD)"
# When called locally with Makefile, the default parameters are used
base="${1:-$(git merge-base "${main_branch}" "${current_branch}")}"
head="${2:-$(git rev-parse HEAD)}"

git diff --diff-filter=ACMRT --name-only "$base" "$head" | docker run -i -v "$(pwd)":/repo eu.gcr.io/freiheit-core/images/execution-plan:2.0-scratch-NG-2 build-pr
