#!/bin/bash
set -ueo pipefail
main_branch="main"
current_branch="$(git rev-parse --abbrev-ref HEAD)"
last_commit_hash="$(git rev-parse HEAD)"
common_ancestor_hash="${1:-$(git merge-base "${main_branch}" "${current_branch}")}"

if [[ "$common_ancestor_hash" = "$last_commit_hash" ]]; then
  # This means that the build is happening on local machine on main branch, so build everything from scratch
  common_ancestor_hash="$(git rev-list --max-parents=0 HEAD)"
fi
git diff --diff-filter=ACMRT --name-only "$common_ancestor_hash"

