#!/bin/sh
set -u
set -e
set -o pipefail
docker run -i -v $(pwd):/repo eu.gcr.io/freiheit-core/services/execution-plan/execution-plan:0.0.2
