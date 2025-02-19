#!/usr/bin/env bash
SCRIPT_DIR="$( cd -- "$( dirname -- "${BASH_SOURCE[0]:-$0}"; )" &> /dev/null && pwd 2> /dev/null; )";
docker build -t kuberpult-builder .
docker run -d --privileged -v "$SCRIPT_DIR"/../../..:/repo kuberpult-builder
id=$(docker ps | grep "kuberpult-builder" | head -n 1 | cut -f1 -d" ")
docker exec "$id" sh -c 'sleep 5; cd /repo/infrastructure/docker/builder; make build; cd /repo/services/frontend-service; make docker; cd /repo/services/cd-service; make docker'
docker kill "$id"
docker rm "$id"

