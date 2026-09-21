#!/usr/bin/env bash

# DISCLAIMER #1: Most likely, your project setup already has an established method to
# make secrets available in your development environment (e.g. direnv). If that is 
# the case, replace / remove this script and use the established method.

# DISCLAIMER #2: Most likely, your project setup already uses a build tool (make) or
# command runner (just). If that is the case, use it to simplify devcontainer setup 
# and run commands and remove this script.

# Wrapper for the `code` CLI that loads this project's per-user secrets first,
# so devcontainer.json's ${localEnv:...} substitutions (in "remoteEnv") see
# them -- without ever exporting them into your interactive shell or typing
# them on the command line.
#
# This matters specifically for VS Code: the Dev Containers extension resolves
# ${localEnv:...} using the environment of the running VS Code process, not of
# whatever terminal you later open inside it. If VS Code was already running
# before you sourced load-env.sh, reopening this folder in a container won't
# see the secrets -- VS Code has to be *launched* with them present, which is
# what this script does.
#
# Safe to run directly, unlike load-env.sh: it only needs the secrets in ITS
# OWN environment before handing off to `code`, which inherits them as a child
# process -- it never needs to export anything back to the shell that ran it.
#
# Usage -- anywhere you'd run `code <args>`, run this instead:
#   .devcontainer/code.sh .
set -euo pipefail

here="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck disable=SC1091
source "${here}/load-env.sh"

exec code "$@"
