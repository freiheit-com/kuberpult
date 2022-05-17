#!/bin/bash
set -ueo pipefail
main_branch="main"
current_branch="$(git rev-parse --abbrev-ref HEAD)"
common_ancestor_hash="${1:-$(git merge-base "${main_branch}" "${current_branch}")}"
last_commit_hash="${2:-$(git rev-parse HEAD)}"

if [[ "$common_ancestor_hash" = "$last_commit_hash" ]]; then
  # This means that the build is happening on local machine on main branch, so build everything from scratch
  # for github actions, this is onlly called from pull request, and pull request base and head commit sha are passed to $common_ancestor_hash and $last_commit_hash
  git ls-files | grep 'Buildfile$'
else
  git diff --diff-filter=ACMRT --name-only "$common_ancestor_hash" "$last_commit_hash"
fi

