#!/bin/bash
set -eu pipefail

# There is no official recent release, so we use the sha of the "latest" version as of now (August 23 2023)
IMAGE='githubchangeloggenerator/github-changelog-generator@sha256:6eceb4caadf513bdfe0d8631c1d70a32398a51d58aa44840b4ced1319fc7b1e6'

docker run -it --rm -v "$(pwd)":/usr/local/src/your-app "${IMAGE}" \
  --user freiheit-com --project kuberpult \
  --token "$TOKEN" \
  --since-tag 1.0.1 \
  --exclude-labels exclude \
  -o CHANGELOG.tmp.md
