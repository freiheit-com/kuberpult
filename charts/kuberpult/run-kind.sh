#!/bin/bash

set -eu
set -o pipefail
set -x

# This script assumes that the docker images have already been built.
# To run/debug/develop this locally, you probably want to run like this:
# rm -rf ./manifests/; make clean; LOCAL_EXECUTION=true ./run-kind.sh

cd "$(dirname $0)"


# prefix every call to "echo" with the name of the script:
function print() {
  /bin/echo "$0:" "$@"
}

cleanup() {
    print "Cleaning stuff up..."
    helm uninstall kuberpult-local || print kuberpult was not installed
    kind delete cluster || print kind cluster was not deleted
}
trap cleanup INT TERM
cleanup

print 'creating kind cluster with a hostpath to share testdata...'
kind create cluster --config=- <<EOF || print cluster exists
kind: Cluster
apiVersion: kind.x-k8s.io/v1alpha4
nodes:
- role: control-plane
EOF

export GIT_NAMESPACE=git

LOCAL_EXECUTION=${LOCAL_EXECUTION:-false}
print "LOCAL_EXECUTION: $LOCAL_EXECUTION"

if "$LOCAL_EXECUTION"
then
  print 'ensuring that the helm chart is build...'
  make all
else
  print 'helm chart must already exist.'
fi

print installing ssh...
./setup-cluster-ssh.sh

function waitForDeployment() {
  ns="$1"
  label="$2"
  print "waiting for $ns/$label"
  until kubectl wait --for=condition=ready pod -n "$ns" -l "$label" --timeout=30s
  do
    sleep 3s
    print ...
  done
}

function portForwardAndWait() {
  ns="$1"
  deployment="$2"
  portHere="$3"
  portThere="$4"
  ports="$portHere:$portThere"
  print "waiting for $ns/$deployment $ports"
  kubectl -n "$ns" port-forward deployment/"$deployment" "$ports" &
  print "waiting until the port forward works..."
  until nc -vz localhost "$portHere"
  do
    sleep 1s
    print ...
  done
}

print "setting up manifest repo"
waitForDeployment "git" "app.kubernetes.io/name=server"
portForwardAndWait "git" "server" "2222" "22"

rm emptyfile -f
print "cloning..."
git config --global user.email 'team.sre.permanent+kuberpult-initial-commiter@freiheit.com'; git config --global user.name 'Initial Kuberpult Commiter';
GIT_SSH_COMMAND='ssh -o UserKnownHostsFile=emptyfile -o StrictHostKeyChecking=no -i ../../services/cd-service/client' git clone ssh://git@localhost:2222/git/repos/manifests

cd manifests
pwd
cp ../../../infrastructure/scripts/create-testdata/testdata_template/environments -r .
git add environments
git commit -m "add initial environments from template"
print "pushing environments to manifest repo..."
GIT_SSH_COMMAND='ssh -o UserKnownHostsFile=emptyfile -o StrictHostKeyChecking=no -i ../../../services/cd-service/client' git push origin master
cd -

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

cd_imagename=${IMAGE_TAG_CD:-$(make --no-print-directory -C ../../services/cd-service/ image-name)}

#cd_imagename=$(make --no-print-directory -C ../../services/cd-service/ image-name)
frontend_imagename=${IMAGE_TAG_FRONTEND:-$(make --no-print-directory -C ../../services/frontend-service/ image-name)}
arr=(${cd_imagename//:/ })
cd_imagename_tag=${arr[1]}
arr=(${frontend_imagename//:/ })
frontend_imagename_tag=${arr[1]}
VERSION=$(make --no-print-directory -C ../../services/cd-service/ version)

print "cd image: $cd_imagename"
print "cd image tag: $cd_imagename_tag"
print "frontend image: $frontend_imagename"
print "frontend image tag: $frontend_imagename_tag"

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
print "$cd_imagename"
print "$frontend_imagename"
kind load docker-image "$cd_imagename"
kind load docker-image "$frontend_imagename"

print 'installing kuberpult helm chart...'

set_options='ingress.domainName=kuberpult.example.com,name=kuberpult-local,VERSION='"$VERSION"',cd.tag='"$cd_imagename_tag",frontend.tag="$frontend_imagename_tag"',git.url=ssh://git@server.'"${GIT_NAMESPACE}"'.svc.cluster.local/git/repos/manifests'
ssh_options=",ssh.identity=$(cat ../../services/cd-service/client),ssh.known_hosts=$(cat ../../services/cd-service/known_hosts),"

helm template ./ --set "$set_options""$ssh_options" > tmp.tmpl
helm install --set "$set_options""$ssh_options" kuberpult-local ./

print 'checking for pods and waiting for portforwarding to be ready...'

kubectl get deployment
kubectl get pods

waitForDeployment "default" "app=kuberpult-frontend-service"
portForwardAndWait "default" "kuberpult-frontend-service" "8081" "8081"
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
