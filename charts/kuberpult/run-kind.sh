#!/bin/bash

set -eu
set -o pipefail
#set -x

# This script assumes that the docker images have already been built

cd "$(dirname $0)"

#(make -C ../../services/cd-service/)


echo starting to install kind
cleanup() {
    echo "Cleaning stuff up..."
    helm uninstall kuberpult-local || echo kuberpult was not installed
    kind delete cluster
}
#trap "trap - SIGTERM && kill -- -$$" SIGINT SIGTERM EXIT
#trap cleanup INT TERM
cleanup


# works
#        command:
#        - ls
#        - /template/

#original:
#        command:
#        - git
#        - init
#        - "--bare"
#        - "/git/repos/manifests"
#
#
#        command:
#        - /bin/sh
#        - "-c"
#        - pwd
#        - echo hello
#        - ls -l /template
#        - git init "--bare" "/git/repos/manifests"
#        - ls -l /template
#

kind create cluster --config=- <<EOF || echo cluster exists
kind: Cluster
apiVersion: kind.x-k8s.io/v1alpha4
nodes:
- role: control-plane
  extraMounts:
   - hostPath: $(pwd)/../../infrastructure/scripts/create-testdata/
     containerPath: /create-testdata
EOF
#kind create cluster || echo cluster already exists

export GIT_NAMESPACE=git

make all

echo installing ssh...
./setup-cluster-ssh.sh

echo installing kuberpult...

export IMAGE_REGISTRY=europe-west3-docker.pkg.dev/fdc-public-docker-registry/kuberpult
WITH_DOCKER=true make -C ../../services/cd-service/ docker
make -C ../../services/frontend-service/ docker


cd_imagename=$(make --no-print-directory -C ../../services/cd-service/ image-name)
frontend_imagename=$(make --no-print-directory -C ../../services/frontend-service/ image-name)
VERSION=$(make --no-print-directory -C ../../services/cd-service/ version)

echo version is "$VERSION"
echo frontend_imagename is "$frontend_imagename"


#docker pull "$cd_imagename"
#docker pull "$frontend_imagename"

kind load docker-image "$cd_imagename"
kind load docker-image "$frontend_imagename"


# ssh://git@server.${GIT_NAMESPACE}.svc.cluster.local/git/repos/manifests
set_options='ingress.domainName=kuberpult.example.com,git.url=git.example.com,name=kuberpult-local,VERSION='"$VERSION"',git.url=ssh://git@server.'"${GIT_NAMESPACE}"'.svc.cluster.local/git/repos/manifests'
ssh_options=",ssh.identity=$(cat ../../services/cd-service/client),ssh.known_hosts=$(cat ../../services/cd-service/known_hosts),"

echo "set options version: $set_options"

helm template ./ --set "$set_options""$ssh_options" > tmp.tmpl
helm install --set "$set_options""$ssh_options" kuberpult-local ./

kubectl get deployment
kubectl get pods

until kubectl wait --for=condition=ready pod -l app=kuberpult-frontend-service --timeout=30s
do
  sleep 3s
  echo ...
done
echo frontend service is up

kubectl port-forward deployment/kuberpult-frontend-service 8081:8081 &

echo "waiting until the port forward works..."
until curl localhost:8081
do
  sleep 1s
  echo ...
done

echo "connection to frontend service successful"

kubectl get deployment
kubectl get pods

sleep 1h