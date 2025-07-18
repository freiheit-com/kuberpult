#!/bin/bash
set -ueo pipefail
script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd -P)"
cd "${script_dir}"
# shellcheck disable=SC1091
source "${script_dir}"/container.inc.sh

echo "input does not matter here, we are building everything anyway" | ./create-matrix.sh build-main
