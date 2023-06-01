#!/bin/bash

set -eu -pipefail
set -x

cd "$(dirname $0)"

echo starting to install kind


#make all

export IMAGE_REGISTRY=europe-west3-docker.pkg.dev/fdc-public-docker-registry/kuberpult
#make -C ../../services/cd-service/ docker
#make -C ../../services/frontend-service/ docker


cd_imagename=$(make --no-print-directory -C ../../services/cd-service/ image-name)
frontend_imagename=$(make --no-print-directory -C ../../services/frontend-service/ image-name)

kind load docker-image "$cd_imagename"
kind load docker-image "$frontend_imagename"

helm uninstall kuberpult-local
set_options='ingress.domainName=kuberpult.example.com,git.url=git.example.com,name=kuberpult-local'
helm template ./ --set "$set_options" > tmp.tmpl
helm install --set "$set_options" kuberpult-local ./


