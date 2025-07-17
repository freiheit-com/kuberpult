#!/bin/bash
## create-matrix.sh MAKE_TARGET
## create-matrix.sh creates the matrix data for github actions.
## It requires the git diff as input, and decides what to build and prints that as json.
## It also tells you why id decided to build something and prints that to stderr.

set -uo pipefail
cd "$(dirname "${BASH_SOURCE[0]}")" || exit 1

STAGE_A_BUILDS="builder deps"
STAGE_B_BUILDS="pkg cli services/cd-service services/frontend-service services/manifest-repo-export-service services/rollout-service services/reposerver-service"

function debug() {
  echo -e debug: "$@" > /dev/stderr
}

function sanitizeArtifactName() {
  # replace chars by _
  echo "$@" | tr /- _
}

function createMatrix() {
  makeTarget=${1}
  ALL_FILES="$(cat)"
  if [ "${makeTarget}" = "build-main" ]
  then
    # if the flag is given, we build everything:
    debug "Building everything, because of ${makeTarget} parameter (main build)."
    for stage in $STAGE_B_BUILDS $STAGE_A_BUILDS
    do
      ALL_FILES=$(echo -e "${ALL_FILES}\n${stage}\n")
    done
  else
    debug "Building only what's required, because of ${makeTarget} parameter (pull-request build)."
  fi

  # if we have pkg, then build all go services
  echo "${ALL_FILES}" | grep '^pkg' -q
  # shellcheck disable=SC2181
  if [ "$?" -eq 0 ]
  then
    debug "pkg was touched, therefore we need to build all of stage B as well."
    for stageB in $STAGE_B_BUILDS
    do
      ALL_FILES=$(echo -e "${ALL_FILES}\n${stageB}\n")
    done
  else
    debug "pkg untouched, no need to build all of stage B"
  fi

  stageArray=""
  for stageADirectory in $STAGE_A_BUILDS
  do
    grepOutput=$(echo "${ALL_FILES}" | grep "infrastructure/docker/${stageADirectory}")
  # shellcheck disable=SC2181
    if [ $? -eq 0 ]
    then
      debug "adding ${stageADirectory} to the list because of a change in $(echo -e "${grepOutput}" | head -n 1)"
      inner=$(jq -n --arg directory "infrastructure/docker/${stageADirectory}" \
                    --arg command "make -C infrastructure/docker/${stageADirectory} ${makeTarget}" \
                    --arg artifacts "" \
                    --arg artifactName "Artifact_infrastructure_docker_${stageADirectory}" \
                    '$ARGS.named'
      )
    else
      debug "adding ${stageADirectory} to the list, despite no change, in order to tag the main image."
      inner=$(jq -n --arg directory "infrastructure/docker/${stageADirectory}" \
                    --arg command "make -C infrastructure/docker/${stageADirectory} retag-main" \
                    --arg artifacts "" \
                    --arg artifactName "Artifact_infrastructure_docker_${stageADirectory}" \
                    '$ARGS.named'
      )
    fi
    if [ -z "${stageArray}" ]
    then
      stageArray="${inner}"
    else
      stageArray="${stageArray},${inner}"
    fi
  done

  if [ -n "${stageArray}" ]
  then
    debug "Stage A was touched, therefore we need to build all of stage B as well."
    for stageB in $STAGE_B_BUILDS
    do
      ALL_FILES=$(echo -e "${ALL_FILES}\n${stageB}\n")
    done
  fi

  stageA=$(jq -n --argjson steps "[$stageArray]" \
                '$ARGS.named'
  )

  stageArray=""
  for stageBDirectory in $STAGE_B_BUILDS
  do
    grepOutput=$(echo "${ALL_FILES}" | grep "${stageBDirectory}")
  # shellcheck disable=SC2181
    if [ $? -eq 0 ]
    then
      debug "adding ${stageBDirectory} to the list because of a change in $(echo -e "${grepOutput}" | head -n 1)"
      inner=$(jq -n --arg directory "${stageBDirectory}" \
                    --arg command "make -C ${stageBDirectory} ${makeTarget}" \
                    --arg artifacts "" \
                    --arg artifactName "Artifact_$(sanitizeArtifactName "${stageBDirectory}")" \
                    '$ARGS.named'
      )
      if [ -z "${stageArray}" ]
      then
        stageArray="${inner}"
      else
        stageArray="${stageArray},${inner}"
      fi
    else
      debug skipping
    fi
  done
  stageB=$(jq -n --argjson steps "[$stageArray]" \
                '$ARGS.named'
  )

  root=$(jq -n --argjson stage_a "$stageA" \
               --argjson stage_b "$stageB" \
                '$ARGS.named'
  )
  echo "$root"
}

createMatrix "${1}"
