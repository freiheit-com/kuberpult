#!/bin/bash
## create-matrix.sh MAKE_TARGET
## create-matrix.sh creates the matrix data for github actions.
## It requires the git diff as input, and decides what to build and prints that as json.
## It also tells you why id decided to build something and prints that to stderr.

set -uo pipefail
cd "$(dirname "${BASH_SOURCE[0]}")" || exit 1

STAGE_A_BUILDS="infrastructure/docker/builder infrastructure/docker/ui"
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
  grepOutput=$(echo "${ALL_FILES}" | grep '^pkg')
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
  debug "grep for pkg: ${grepOutput}"

  # if we have a change in go.mod, we need to rebuild the builder image:
  grepOutput=$(echo "${ALL_FILES}" | grep '^go.mod')
  # shellcheck disable=SC2181
  if [ "$?" -eq 0 ]
  then
    debug "go.mod was touched, therefore we need to build the builder as well."
    for stageA in $STAGE_A_BUILDS
    do
      ALL_FILES=$(echo -e "${ALL_FILES}\n${stageA}\n")
    done
  else
    debug "go.mod untouched, no need to build all of stage B"
  fi
  debug "grep for go.mod: ${grepOutput}"

  stageArray=""
  stageArrayFullBuild=false
  for stageADirectory in $STAGE_A_BUILDS
  do
    grepOutput=$(echo "${ALL_FILES}" | grep "^${stageADirectory}")
  # shellcheck disable=SC2181
    if [ $? -eq 0 ] || [ "${makeTarget}" = "build-main" ]
    then
      debug "adding stage-a directory ${stageADirectory} to the list because of a change in $(echo -e "${grepOutput}" | head -n 1) OR ${makeTarget}==build-main"
      inner=$(jq -n --arg directory "${stageADirectory}" \
                    --arg command "make -C ${stageADirectory} ${makeTarget}" \
                    --arg artifacts "" \
                    --arg artifactName "Artifact_infrastructure_docker_${stageADirectory}" \
                    '$ARGS.named'
      )
      stageArrayFullBuild=true
    else
      debug "adding ${stageADirectory} to the list, despite no change, in order to tag the main image."
      inner=$(jq -n --arg directory "${stageADirectory}" \
                    --arg command "make -C ${stageADirectory} pull-main build-pr" \
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
    debug "grep stage a: ${grepOutput}"
  done

  if [ -n "${stageArray}" ] && ${stageArrayFullBuild}
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
    grepOutput=$(echo "${ALL_FILES}" | grep "^${stageBDirectory}")
  # shellcheck disable=SC2181
    if [ $? -eq 0 ]
    then
      debug "adding stage-b directory ${stageBDirectory} to the list because of a change in $(echo -e "${grepOutput}" | head -n 1)"
      inner=$(jq -n --arg directory "${stageBDirectory}" \
                    --arg command "make -C ${stageBDirectory} ${makeTarget}" \
                    --arg artifacts "" \
                    --arg artifactName "Artifact_$(sanitizeArtifactName "${stageBDirectory}")" \
                    '$ARGS.named'
      )
    else
      debug "adding ${stageBDirectory} to the list, despite no change, in order to tag the main image."
      inner=$(jq -n --arg directory "${stageBDirectory}" \
                    --arg command "make -C ${stageBDirectory} pull-main build-pr" \
                    --arg artifacts "" \
                    --arg artifactName "Artifact_$(sanitizeArtifactName "${stageBDirectory}")" \
                    '$ARGS.named'
      )
    fi
    debug "grep stage b: ${grepOutput}"
    if [ -z "${stageArray}" ]
    then
      stageArray="${inner}"
    else
      stageArray="${stageArray},${inner}"
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
