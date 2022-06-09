#!/bin/bash
set -euo pipefail
# Pushes a docker image only if it does not exist, print an error message if it exists

IMAGE_URL=$1

if [[ "${IMAGE_URL}" == "" ]];
then
    echo "Please provide image URL as an argument"
    exit 1
fi

RET=$(DOCKER_CLI_EXPERIMENTAL=enabled docker manifest inspect ${IMAGE_URL} > /dev/null; echo $?)
if [ "${RET}" == "1" ];
then
    docker push ${IMAGE_URL}
else
    echo "Image ${IMAGE_URL} already exists, please update the version in Buildfile and push again"
fi
