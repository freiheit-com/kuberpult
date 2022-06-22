#!/bin/bash
set -ueo pipefail
script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd -P)"
# yaml file override keeps comments and removes whitelines, so lint fails. Removing comments for lint
grep -o '^[^#]*' "$script_dir"/../values.yaml > "$script_dir"/test-values.yaml
yq eval-all -i 'select(fileIndex == 0) * select(fileIndex == 1)' "$script_dir"/test-values.yaml "$script_dir"/test-values-override.yaml 
