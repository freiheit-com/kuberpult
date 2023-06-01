#!/bin/bash

set -eu
set -o pipefail
set -x

cd "$(dirname $0)"

echo starting to install kind


#make all

export IMAGE_REGISTRY=europe-west3-docker.pkg.dev/fdc-public-docker-registry/kuberpult
#make -C ../../services/cd-service/ docker
#make -C ../../services/frontend-service/ docker


cd_imagename=$(make --no-print-directory -C ../../services/cd-service/ image-name)
frontend_imagename=$(make --no-print-directory -C ../../services/frontend-service/ image-name)

docker pull "$cd_imagename"
docker pull "$frontend_imagename"

kind load docker-image "$cd_imagename"
kind load docker-image "$frontend_imagename"

set_options='ingress.domainName=kuberpult.example.com,git.url=git.example.com,name=kuberpult-local'
helm template ./ --set "$set_options" > tmp.tmpl
helm install --set "$set_options" kuberpult-local ./

kubectl get deployment
kubectl get pods



sleep 30s
helm uninstall kuberpult-local || echo "could not uninstall helm chart for kuberpult"
