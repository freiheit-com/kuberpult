#!/bin/bash
## create-matrix.sh MAKE_TARGET
## create-matrix.sh creates the matrix data for github actions
## It requires the git diff as input, and decides what to build

set -uo pipefail
cd "$(dirname "${BASH_SOURCE[0]}")"

STAGE_A_BUILDS="builder deps"
STAGE_B_BUILDS="services/cd-service services/frontend-service pkg"

function debug() {
  echo $@ > /dev/null
}

function sanitizeArtifactName() {
  # replace chars by _
  echo $@ | tr /- _
}

function createMatrix() {
  ALL_FILES="$(cat)"

  makeTarget=${1}

  stageArray=""
  for stageADirectory in $STAGE_A_BUILDS
  do
    debug -e "\n\nStage A: $stageADirectory"
    echo "${ALL_FILES}" | grep "infrastructure/docker/${stageADirectory}" > /dev/null
    if [ $? -eq 0 ]
    then
      debug found
      inner=$(jq -n --arg directory "infrastructure/docker/${stageADirectory}" \
                    --arg command "make -C infrastructure/docker/${stageADirectory} ${makeTarget}" \
                    --arg artifacts "" \
                    --arg artifactName "Artifact_infrastructure_docker_${stageADirectory}" \
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
  stageA=$(jq -n --argjson steps "[$stageArray]" \
                '$ARGS.named'
  )

  stageArray=""
  for stageBDirectory in $STAGE_B_BUILDS
  do
    debug -e "\n\nStage B: $stageBDirectory"
    echo "${ALL_FILES}" | grep "${stageBDirectory}" > /dev/null
    if [ $? -eq 0 ]
    then
      debug found
      inner=$(jq -n --arg directory "${stageBDirectory}" \
                    --arg command "make -C ${stageBDirectory} ${makeTarget}" \
                    --arg artifacts "" \
                    --arg artifactName "Artifact_$(sanitizeArtifactName ${stageBDirectory})" \
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


  root=$(jq -n --argjson stage_a "[$stageA]" \
               --argjson stage_b "[$stageB]" \
                '$ARGS.named'
  )
  echo "$root"
}


createMatrix "${1}"
