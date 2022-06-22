#!/bin/bash
set -ueo pipefail
script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd -P)"
# yaml file override can't be used because as of now, yq can't remove comments, or preserve whitespace. So lint fails.
yq e -i '.git.url = "/repository"' "$script_dir"/test-values.yaml
yq e -i '.ingress.domainName = "example.kuberpult.com"' "$script_dir"/test-values.yaml
yq e -i '.cd.resources.requests.cpu = "500m"' "$script_dir"/test-values.yaml
yq e -i '.cd.resources.limits.cpu = "500m"' "$script_dir"/test-values.yaml
yq e -i '.cd.resources.requests.memory = "1Gi"' "$script_dir"/test-values.yaml
yq e -i '.cd.resources.limits.memory = "1Gi"' "$script_dir"/test-values.yaml
yq e -i '.fronted.resources.limits.cpu = "250m"' "$script_dir"/test-values.yaml
yq e -i '.fronted.resources.requests.cpu = "250m"' "$script_dir"/test-values.yaml
