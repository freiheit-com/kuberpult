#!/usr/bin/env bash
# Loads this project's environment variables into the CURRENT shell's
# environment, so devcontainer.json's ${localEnv:...} substitutions (in
# "remoteEnv") see them when you run `devcontainer up` / `devcontainer exec`
# next.
#
# Two files are loaded, in order:
#   - project.env, alongside this script: non-secret, project-scoped defaults
#     (e.g. the Revolution workspace/product IDs). Committed to the repo, so
#     every checkout gets the same values without per-user setup.
#   - the per-user secrets file below: host-local and never committed. Populate
#     it with plain KEY=value lines (see .devcontainer/README.md for which
#     keys). Loaded second so a secret could in principle override a default.
#
# Must be sourced, not executed.
#
# Usage, once per shell session, from the repository root:
#   source .devcontainer/load-env.sh

sourced=true
if [ -n "${BASH_SOURCE:-}" ]; then
    [ "${BASH_SOURCE[0]}" = "${0}" ] && sourced=false
elif [ -n "${ZSH_EVAL_CONTEXT:-}" ]; then
    case "${ZSH_EVAL_CONTEXT}" in
    *:file) sourced=true ;;
    *) sourced=false ;;
    esac
fi

if [ "${sourced}" != "true" ]; then
    echo "load-env.sh must be sourced, not executed: run 'source ${0}' instead." >&2
    exit 1
fi

# ${BASH_SOURCE[0]} tracks this file's own path in bash even when sourced; zsh
# has no such array, but (per FUNCTION_ARGZERO, on by default) sets $0 to the
# sourced file's path instead, so it's the right fallback.
here="$(cd "$(dirname "${BASH_SOURCE[0]:-${0}}")" && pwd)"

project_env_file="${here}/project.env"

set -a
# shellcheck source=/dev/null
. "${project_env_file}"
set +a

# PROJECT_SECRETS_FILE comes from project.env -- see the comment there. Set
# explicitly rather than derived, so a copy of this directory into a new project fails loudly here.
: "${PROJECT_SECRETS_FILE:?PROJECT_SECRETS_FILE is not set -- check ${project_env_file}}"

if [ -f "${PROJECT_SECRETS_FILE}" ]; then
    set -a
    # shellcheck source=/dev/null
    . "${PROJECT_SECRETS_FILE}"
    set +a
else
    echo "load-env.sh: no file at ${PROJECT_SECRETS_FILE}, nothing loaded from it." >&2
fi

unset here project_env_file sourced PROJECT_SECRETS_FILE
