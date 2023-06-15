#!/usr/bin/env bash
set -eou pipefail
out=$(nix build --no-link --json "$1" | jq -r '.[0].outputs.out')
$out | docker load
