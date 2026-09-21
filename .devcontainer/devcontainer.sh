#!/usr/bin/env bash

# DISCLAIMER #1: Most likely, your project setup already has an established method to
# make secrets available in your development environment (e.g. direnv). If that is 
# the case, replace / remove this script and use the established method.

# DISCLAIMER #2: Most likely, your project setup already uses a build tool (make) or
# command runner (just). If that is the case, use it to simplify devcontainer setup 
# and run commands and remove this script.

# Wrapper for the `devcontainer` CLI that loads this project's per-user secrets
# first, so devcontainer.json's ${localEnv:...} substitutions (in "remoteEnv")
# see them -- without ever exporting them into your interactive shell or
# typing them on the command line.
#
# Safe to run directly, unlike load-env.sh: it only needs the secrets in ITS
# OWN environment before handing off to `devcontainer`, which inherits them as
# a child process -- it never needs to export anything back to the shell that
# ran it.
#
# Usage -- anywhere you'd run `devcontainer <args>`, run this instead:
#   .devcontainer/devcontainer.sh up --workspace-folder .
#   .devcontainer/devcontainer.sh exec --workspace-folder . zsh
set -euo pipefail

here="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck disable=SC1091
source "${here}/load-env.sh"

exec devcontainer "$@"
