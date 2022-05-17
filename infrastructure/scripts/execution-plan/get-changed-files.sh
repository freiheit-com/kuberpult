#!/bin/bash
set -ueo pipefail
main_branch="main"
current_branch="$(git rev-parse --abbrev-ref HEAD)"

if [[ "$main_branch" = "$current_branch" ]]; then
    # In main branch find the diff between current and previous commit only 
    common_ancestor_hash="HEAD^"
else
    # Running for pull request, find the common ancestor and find changes till present
    common_ancestor_hash="$(git merge-base "${main_branch}" "${current_branch}")"
fi
git diff --diff-filter=ACMRT --name-only "$common_ancestor_hash"

