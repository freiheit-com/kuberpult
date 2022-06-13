#!/bin/bash
set -ueo pipefail
SCRIPT_DIR="$( cd -- "$( dirname -- "${BASH_SOURCE[0]:-$0}"; )" &> /dev/null && pwd 2> /dev/null; )";
find -name Buildfile | docker run -i -v "$(pwd)":/repo $($SCRIPT_DIR/container.sh) build-main
