#!/bin/bash

set -eu
set -o pipefail
#set -x

# This script assumes that the docker images have already been built.
# To run/debug/develop this locally, you probably want to run like this:
# make clean; LOCAL_EXECUTION=true ./run-kind.sh

cd "$(dirname $0)"

#(make -C ../../services/cd-service/)

# prefix every call to "echo" with the name of the script:
function print() {
  /bin/echo "$0:" "$@"
}

cleanup() {
    print "Cleaning stuff up..."
    helm uninstall kuberpult-local || print kuberpult was not installed
    kind delete cluster || print kind cluster was not deleted
}
#trap "trap - SIGTERM && kill -- -$$" SIGINT SIGTERM EXIT
trap cleanup INT TERM
cleanup

print 'creating kind cluster with a hostpath to share testdata...'
kind create cluster --config=- <<EOF || print cluster exists
kind: Cluster
apiVersion: kind.x-k8s.io/v1alpha4
nodes:
- role: control-plane
  extraMounts:
   - hostPath: $(pwd)/../../infrastructure/scripts/create-testdata/
     containerPath: /create-testdata
EOF

export GIT_NAMESPACE=git

LOCAL_EXECUTION=${LOCAL_EXECUTION:-false}
print "LOCAL: $LOCAL_EXECUTION"


print 'ensuring that the helm chart is build...'
make all

print installing ssh...
./setup-cluster-ssh.sh

print installing kuberpult...

export IMAGE_REGISTRY=europe-west3-docker.pkg.dev/fdc-public-docker-registry/kuberpult

if "$LOCAL_EXECUTION"
then
  print 'building cd service...'
  WITH_DOCKER=true make -C ../../services/cd-service/ docker

  print 'building frontend service...'
  make -C ../../services/frontend-service/ docker
else
  print 'not building cd or frontend service...'
fi

cd_imagename=$(make --no-print-directory -C ../../services/cd-service/ image-name)
frontend_imagename=$(make --no-print-directory -C ../../services/frontend-service/ image-name)
VERSION=$(make --no-print-directory -C ../../services/cd-service/ version)


if ! "$LOCAL_EXECUTION"
then
  print 'pulling cd service...'
  docker pull "$cd_imagename"
  print 'pulling frontend service...'
  docker pull "$frontend_imagename"
else
  print 'not pulling cd or frontend service...'
fi

print 'loading docker images into kind...'
kind load docker-image "$cd_imagename"
kind load docker-image "$frontend_imagename"

print 'installing kuberpult helm chart...'

set_options='ingress.domainName=kuberpult.example.com,git.url=git.example.com,name=kuberpult-local,VERSION='"$VERSION"',git.url=ssh://git@server.'"${GIT_NAMESPACE}"'.svc.cluster.local/git/repos/manifests'
ssh_options=",ssh.identity=$(cat ../../services/cd-service/client),ssh.known_hosts=$(cat ../../services/cd-service/known_hosts),"

helm template ./ --set "$set_options""$ssh_options" > tmp.tmpl
helm install --set "$set_options""$ssh_options" kuberpult-local ./

print 'checking for pods and waiting for portforwarding to be ready...'

kubectl get deployment
kubectl get pods

until kubectl wait --for=condition=ready pod -l app=kuberpult-frontend-service --timeout=30s
do
  sleep 3s
  print ...
done
print frontend service is up

kubectl port-forward deployment/kuberpult-frontend-service 8081:8081 &

print "waiting until the port forward works..."
until curl localhost:8081
do
  sleep 1s
  print ...
done

print "connection to frontend service successful"

kubectl get deployment
kubectl get pods

if "$LOCAL_EXECUTION"
then
  echo "sleeping for 1h to allow debugging"
  sleep 1h
else
  echo "done. Kind cluster is up and kuberpults frontend service is running."
fi