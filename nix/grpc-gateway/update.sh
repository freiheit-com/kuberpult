#!/usr/bin/env bash
set -eou pipefail

d=$(mktemp -d)
trap "exit 1"      HUP INT PIPE QUIT TERM
trap 'rm -rf "$d"' EXIT

pushd "$d"
git clone git@github.com:grpc-ecosystem/grpc-gateway .
rev=$(git rev-list --tags --max-count=1)
tag=$(git describe --tags "$rev")
git checkout "$rev"
popd

gomod2nix --dir "$d" --outdir .
cat <<JSON > lock.json
{"tag":"$tag","rev":"$rev"}
JSON
