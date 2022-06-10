#!/bin/bash
set -ueo pipefail
find -name Buildfile | docker run -i -v "$(pwd)":/repo eu.gcr.io/freiheit-core/images/execution-plan:2.0-scratch-NG-6 build-main
