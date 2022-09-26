#!/bin/bash

set -eo pipefail

DRY_RUN=0
if [ "$1" == "--dry-run" ];
then
    DRY_RUN=1
    shift
fi

set -u

FROM_REVISION="$1"
PROJECT=""

if [ ! -z ${CODE_REVIEWER_PROJECT:-} ]; then
    PROJECT="--project ${CODE_REVIEWER_PROJECT}"
fi

if [ $DRY_RUN -eq 1 ]; then
    @"${CODE_REVIEWER_LOCATION}" analyse multiple . --commit "${FROM_REVISION}" --config codereviewr.yaml ${PROJECT} --dry-run
else
    @"${CODE_REVIEWER_LOCATION}" analyse multiple . --commit "${FROM_REVISION}" --config codereviewr.yaml ${PROJECT}
fi
