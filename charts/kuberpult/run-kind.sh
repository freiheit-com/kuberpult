#!/usr/bin/env bash

# shellcheck source=/dev/null
source "$(dirname "$0")/lib.sh"

set -eu
set -o pipefail

# This script assumes that the docker images have already been built.
# To run/debug/develop this locally, you probably want to run like this:
# rm -rf ./manifests/; make clean; LOCAL_EXECUTION=true ./run-kind.sh

cd "$(dirname "$0")"



cleanup() {
    print "Cleaning stuff up..."
    helm uninstall kuberpult-local || print kuberpult was not installed
    kind delete cluster || print kind cluster was not deleted
}
cleanup

print 'creating kind cluster with a hostpath to share testdata...'
kind create cluster --config=- <<EOF || print cluster exists
kind: Cluster
apiVersion: kind.x-k8s.io/v1alpha4
nodes:
- role: control-plane
EOF

export GIT_NAMESPACE=git
export ARGO_NAMESPACE=default

LOCAL_EXECUTION=${LOCAL_EXECUTION:-true}
print "LOCAL_EXECUTION: $LOCAL_EXECUTION"

print 'ensuring that the helm chart is build...'
# it was already build, but we are in another workflow now, so we have to rebuild it
make all

print installing ssh...
./setup-cluster-ssh.sh

print installing postgres...
./setup-postgres.sh

GPG="gpg --keyring trustedkeys-kuberpult.gpg"
gpgFile=~/.gnupg/trustedkeys-kuberpult.gpg
if test -f "$gpgFile"
then
  echo warning: file already exists: "$gpgFile"
  if "$LOCAL_EXECUTION"
  then
    echo "is it ok to delete the file? Press enter twice to delete"
    # shellcheck disable=SC2162
#    read
    # shellcheck disable=SC2162
#    read
    rm "$gpgFile"
  else
    echo "this file should not exist on the ci"
    exit 1
  fi
fi
$GPG --no-default-keyring --batch --passphrase '' --quick-gen-key kuberpult-kind@example.com
$GPG --armor --export kuberpult-kind@example.com > kuberpult-keyring.gpg

print "setting up manifest repo"
waitForDeployment "git" "app.kubernetes.io/name=server"
portForwardAndWait "git" "deployment/server" "2222" "22"

rm -f emptyfile
rm -rf manifests
print "cloning..."
GIT_SSH_COMMAND='ssh -o UserKnownHostsFile=emptyfile -o StrictHostKeyChecking=no -i ../../services/cd-service/client' git clone ssh://git@localhost:2222/git/repos/manifests

cd manifests
pwd
print 'to run git commands here, run:'
print "export GIT_SSH_COMMAND='ssh -o UserKnownHostsFile=emptyfile -o StrictHostKeyChecking=no -i ../../../services/cd-service/client'"

cp -r ../../../infrastructure/scripts/create-testdata/testdata_template/environments .
git add environments
GIT_AUTHOR_NAME='Initial Kuberpult Commiter' GIT_COMMITTER_NAME='Initial Kuberpult Commiter' GIT_AUTHOR_EMAIL='team.sre.permanent+kuberpult-initial-commiter@freiheit.com'  GIT_COMMITTER_EMAIL='team.sre.permanent+kuberpult-initial-commiter@freiheit.com' git commit -m "add initial environments from template"
print "pushing environments to manifest repo..."
GIT_SSH_COMMAND='ssh -o UserKnownHostsFile=emptyfile -o StrictHostKeyChecking=no -i ../../../services/cd-service/client' git checkout -B main
GIT_SSH_COMMAND='ssh -o UserKnownHostsFile=emptyfile -o StrictHostKeyChecking=no -i ../../../services/cd-service/client' git push -f origin main
cd -


export IMAGE_REGISTRY=europe-west3-docker.pkg.dev/fdc-public-docker-registry/kuberpult
print version...
VERSION=$(make --no-print-directory -C ../../ version)
print "version is ${VERSION}"
IMAGE_TAG_KUBERPULT=${IMAGE_TAG_KUBERPULT:-$VERSION}
print "IMAGE_TAG_KUBERPULT is now ${IMAGE_TAG_KUBERPULT}"

if "$LOCAL_EXECUTION"
then
  print 'building services...'
  IMAGE_TAG=$IMAGE_TAG_KUBERPULT make -C ../../infrastructure/docker/builder build
  IMAGE_TAG=$IMAGE_TAG_KUBERPULT make -j5 -C ../../charts/kuberpult build-all-docker
else
  print 'not building services...'
fi

cd_imagename="${IMAGE_REGISTRY}/kuberpult-cd-service:${IMAGE_TAG_KUBERPULT}"
manifest_repo_export_imagename="${IMAGE_REGISTRY}/kuberpult-manifest-repo-export-service:${IMAGE_TAG_KUBERPULT}"
frontend_imagename="${IMAGE_REGISTRY}/kuberpult-frontend-service:${IMAGE_TAG_KUBERPULT}"
rollout_imagename="${IMAGE_REGISTRY}/kuberpult-rollout-service:${IMAGE_TAG_KUBERPULT}"
reposerver_imagename="${IMAGE_REGISTRY}/kuberpult-reposerver-service:${IMAGE_TAG_KUBERPULT}"

print "cd image: $cd_imagename"
print "frontend image: $frontend_imagename"
print "rollout image: $rollout_imagename"

if ! "$LOCAL_EXECUTION"
then
  print 'pulling cd service...'
  docker pull "$cd_imagename"
  print 'pulling manifest_repo_export service...'
  docker pull "$manifest_repo_export_imagename"
  print 'pulling frontend service...'
  docker pull "$frontend_imagename"
  print 'pulling rollout service...'
  docker pull "$rollout_imagename"
  print 'pulling reposerver service...'
  docker pull "$reposerver_imagename"
else
  print 'not pulling cd or frontend service...'
fi

ARGOCD_IMAGE_URI="quay.io/argoproj/argocd:v2.7.4"
DEX_IMAGE_URI="ghcr.io/dexidp/dex:v2.36.0"
CLOUDSQL_PROXY_IMAGE_URI="gcr.io/cloud-sql-connectors/cloud-sql-proxy:2.11.0"
REDIS_IMAGE_URI="public.ecr.aws/docker/library/redis:7.0.11-alpine"

print 'pulling argocd image...'
docker pull "$ARGOCD_IMAGE_URI"

print 'pulling dex image...'
docker pull "$DEX_IMAGE_URI"

print 'pulling cloudsql proxy image...'
docker pull "$CLOUDSQL_PROXY_IMAGE_URI"

print 'pulling redis image...'
docker pull "$REDIS_IMAGE_URI"

print 'loading docker images into kind...'
print "$cd_imagename"
print "$frontend_imagename"
(
  for image in "$cd_imagename" "$manifest_repo_export_imagename" "$rollout_imagename" "$reposerver_imagename" "$frontend_imagename" "$ARGOCD_IMAGE_URI" "$DEX_IMAGE_URI" "$CLOUDSQL_PROXY_IMAGE_URI" "$REDIS_IMAGE_URI"
  do
    kind load docker-image "${image}" &
  done
  print "waiting to load all docker images..."
  wait
)

## argoCd

print "starting argoCd..."

helm repo add argo-cd https://argoproj.github.io/argo-helm


reposerver_service=kuberpult-reposerver-service
namespace=default
helm uninstall argocd || echo "did not uninstall argo"
cat <<YAML > argocd-values.yml
repoServer:
  replicas: 0
configs:
  params:
    repo.server: ${reposerver_service}.${namespace}.svc.cluster.local:8443
    controller.repo.server.plaintext: true
    server.repo.server.plaintext: true
    applicationsetcontroller.log.level: warn
  ssh:
    knownHosts: |
$(sed -e "s/^/        /" <../../services/cd-service/known_hosts)
  cm:
    accounts.kuberpult: apiKey
    timeout.reconciliation: 0s
  rbac:
    policy.csv: |
      p, role:kuberpult, applications, get, */*, allow
      p, role:kuberpult, applications, create, */*, allow
      p, role:kuberpult, applications, update, */*, allow
      p, role:kuberpult, applications, sync, */*, allow
      p, role:kuberpult, applications, delete, */*, allow
      g, kuberpult, role:kuberpult

YAML

echo "installing argocd $(cat ./argocd-values.yml)"

helm install argocd argo-cd/argo-cd --values argocd-values.yml --version 5.36.0

print applying app...

#waitForDeployment "default" "app.kubernetes.io/name=argocd-repo-server"
waitForDeployment "default" "app.kubernetes.io/name=argocd-server"
portForwardAndWait "default" service/argocd-server 8080 443

# For now, we are only creating development here
# This means argo cd will only handle development, including the rollout-status
kubectl apply -f - <<EOF
apiVersion: argoproj.io/v1alpha1
kind: AppProject
metadata:
  name: development
  namespace: ${ARGO_NAMESPACE}
spec:
  description: test-env
  destinations:
  - name: "dest1"
    namespace: '*'
    server: https://kubernetes.default.svc
  sourceRepos:
  - '*'
---
apiVersion: argoproj.io/v1alpha1
kind: AppProject
metadata:
  name: dev
  namespace: ${ARGO_NAMESPACE}
spec:
  description: test-env
  destinations:
  - name: "dest1"
    namespace: '*'
    server: https://kubernetes.default.svc
  sourceRepos:
  - '*'
---
apiVersion: argoproj.io/v1alpha1
kind: AppProject
metadata:
  name: staging
  namespace: ${ARGO_NAMESPACE}
spec:
  description: staging-normal
  destinations:
  - name: "dest1"
    namespace: '*'
    server: https://kubernetes.default.svc
  sourceRepos:
  - '*'
---
apiVersion: argoproj.io/v1alpha1
kind: AppProject
metadata:
  name: aa-aa-test-dev-1
  namespace: ${ARGO_NAMESPACE}
spec:
  description: aa-aa-test-dev-1
  destinations:
  - name: "dest1"
    namespace: '*'
    server: https://kubernetes.default.svc
  sourceRepos:
  - '*'
---
apiVersion: argoproj.io/v1alpha1
kind: AppProject
metadata:
  name: aa-aa-test-dev-2
  namespace: ${ARGO_NAMESPACE}
spec:
  description: aa-aa-test-dev-2
  destinations:
  - name: "dest1"
    namespace: '*'
    server: https://kubernetes.default.svc
  sourceRepos:
  - '*'
---
# root app is only necessary and desired if we use git with the manifest-repo-export
#apiVersion: argoproj.io/v1alpha1
#kind: Application
#metadata:
#  name: root
#  namespace: ${ARGO_NAMESPACE}
#spec:
#  destination:
#    namespace: ${ARGO_NAMESPACE}
#    server: https://kubernetes.default.svc
#  project: dev
#  source:
#    path: argocd/v1alpha1
#    repoURL: ssh://git@server.${GIT_NAMESPACE}.svc.cluster.local/git/repos/manifests
#    targetRevision: main
#  syncPolicy:
#    automated: {}
EOF

print "admin password:"
argocd_adminpw=$(kubectl -n default get secret argocd-initial-admin-secret -o jsonpath="{.data.password}" | base64 -d)
echo "$argocd_adminpw"
echo "$argocd_adminpw" > argocd_adminpw.txt

argocd login localhost:8080 --username admin --password "$argocd_adminpw" --insecure



kubectl create ns development
kubectl create ns development2
kubectl create ns staging
kubectl create ns aa-test


export GIT_NAMESPACE=${GIT_NAMESPACE}
export ARGO_NAMESPACE=${ARGO_NAMESPACE}
export LOCAL_EXECUTION=${LOCAL_EXECUTION}

./install-kuberpult-helm.sh

print 'checking for pods and waiting for portforwarding to be ready...'

kubectl get deployment
kubectl get pods

(cd ../../infrastructure/scripts/create-testdata/ ; sh create-environments.sh)

START=30
NUM_RELEASES=2
END=$(($START + $NUM_RELEASES))
for v in $(seq "$START" "$END")
do
   RELEASE_VERSION=$v ../../infrastructure/scripts/create-testdata/create-release-allparams.sh echo-1 sreteam e
   RELEASE_VERSION=$v ../../infrastructure/scripts/create-testdata/create-release-allparams.sh echo-2 sreteam e
   RELEASE_VERSION=$v ../../infrastructure/scripts/create-testdata/create-release-allparams.sh foo-1 sreteam f
done
../../infrastructure/scripts/create-testdata/run-releasetrain.sh staging
v=$((v + 1))
RELEASE_VERSION=$v ../../infrastructure/scripts/create-testdata/create-release-allparams.sh echo-1 sreteam e
RELEASE_VERSION=$v ../../infrastructure/scripts/create-testdata/create-release-allparams.sh echo-2 sreteam e
RELEASE_VERSION=$v ../../infrastructure/scripts/create-testdata/create-release-allparams.sh foo-1 sreteam f



if false;
then
  for v in $(seq 1 3)
  do
     RELEASE_VERSION=$v ../../infrastructure/scripts/create-testdata/create-release-allparams.sh echo-3 sreteam e
     RELEASE_VERSION=$v ../../infrastructure/scripts/create-testdata/create-release-allparams.sh foo-1 sreteam f

     #RELEASE_VERSION=$v ../../infrastructure/scripts/create-testdata/create-release-allparams.sh foo-1 sreteam f
  done
fi

print "running bracket stability integration tests..."
(cd ../../ && go test -v ./tests/kind-brackets/ -timeout 20m)

if "$LOCAL_EXECUTION"
then
  echo "hit ctrl+c to stop"
  read -r -d '' _ </dev/tty
else
  echo "done. Kind cluster is up and kuberpult and argoCd are running."
fi
