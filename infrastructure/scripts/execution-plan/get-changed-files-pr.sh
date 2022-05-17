#!/bin/bash
set -ueo pipefail
common_ancestor_hash="${1}"
last_commit_hash="${2}"

git diff --diff-filter=ACMRT --name-only "$common_ancestor_hash" "$last_commit_hash"

