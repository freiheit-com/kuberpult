#!/bin/bash
set -ueo pipefail
script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd -P)"
# shellcheck disable=SC1091
source "${script_dir}"/container.inc.sh
find . -name Buildfile.yaml | docker run -i -v "$(pwd)":/repo "${BUILDER_IMAGE}" main
