#!/usr/bin/env bash

# shellcheck source=/dev/null
source "$(dirname "$0")/lib.sh"

set -eu
set -o pipefail

# avoid infinite trap invocations in shell by resetting trap handler on trap:
trap 'trap - EXIT SIGINT SIGTERM; kill 0' EXIT SIGINT SIGTERM

# This script assumes that the docker images have already been built.
# To run/debug/develop this locally, you probably want to run like this:
# rm -rf ./manifests/; make clean; LOCAL_EXECUTION=true GO_TEST_ARGS='' ./run-kind.sh

cd "$(dirname "$0")"

if [ -n "$(git status --porcelain)" ]; then
    print "WARNING: working tree has uncommitted changes. Commit or stash them before running this script."
    git status
    print "WARNING: You may continue safely, if you did not change any SOURCE code, but only tests and scripts."
    sleep 3; exit 1
fi

CLUSTER_EXISTS=false
if kind get clusters 2>/dev/null | grep -q '^kind$'; then
    print "Kind cluster 'kind' already exists, will skip cluster setup steps."
    CLUSTER_EXISTS=true
fi

cleanup() {
    print "Cleaning stuff up..."
    helm uninstall kuberpult-local || print kuberpult was not installed
    kind delete cluster || print kind cluster was not deleted
}

export GIT_NAMESPACE=git
export ARGO_NAMESPACE=default

LOCAL_EXECUTION=${LOCAL_EXECUTION:-true}
print "LOCAL_EXECUTION: $LOCAL_EXECUTION"

if ! "$CLUSTER_EXISTS"; then
    cleanup

    print 'creating kind cluster with a hostpath to share testdata...'
    kind create cluster --config=- <<EOF
kind: Cluster
apiVersion: kind.x-k8s.io/v1alpha4
nodes:
- role: control-plane
EOF

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
    portForwardAndWait "git" "deployment/server" "5000" "22"

    rm -f emptyfile
    rm -rf manifests
    print "cloning..."
    GIT_SSH_COMMAND='ssh -o UserKnownHostsFile=emptyfile -o StrictHostKeyChecking=no -i ../../services/cd-service/client' git clone ssh://git@localhost:5000/git/repos/manifests

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

    print 'loading external docker images into kind...'
    (
      for image in "$ARGOCD_IMAGE_URI" "$DEX_IMAGE_URI" "$CLOUDSQL_PROXY_IMAGE_URI" "$REDIS_IMAGE_URI"
      do
        kind load docker-image "${image}" &
      done
      print "waiting to load all external docker images..."
      wait
    )
fi

export IMAGE_REGISTRY=europe-west3-docker.pkg.dev/fdc-public-docker-registry/kuberpult
print version...
VERSION=$(make --no-print-directory -C ../../ version)
print "version is ${VERSION}"
IMAGE_TAG_KUBERPULT=${IMAGE_TAG_KUBERPULT:-$VERSION}
print "IMAGE_TAG_KUBERPULT is now ${IMAGE_TAG_KUBERPULT}"
frontend_imagename="${IMAGE_REGISTRY}/kuberpult-frontend-service:${IMAGE_TAG_KUBERPULT}"

if "$LOCAL_EXECUTION"
then
  print 'building services...'
  IMAGE_TAG=$IMAGE_TAG_KUBERPULT make -C ../../infrastructure/docker/builder build

  BUILD_FRONTEND=true
  if docker image inspect "$frontend_imagename" > /dev/null 2>&1; then
    print "frontend image already exists ($frontend_imagename), skipping build"
    BUILD_FRONTEND=false
  else
    EXISTING_FRONTEND=$(docker images --format "{{.Repository}}:{{.Tag}}" "${IMAGE_REGISTRY}/kuberpult-frontend-service" | head -1)
    if [ -n "$EXISTING_FRONTEND" ]; then
      print "retagging existing frontend image $EXISTING_FRONTEND -> $frontend_imagename"
      docker tag "$EXISTING_FRONTEND" "$frontend_imagename"
      BUILD_FRONTEND=false
    fi
  fi

  if "$BUILD_FRONTEND"; then
    IMAGE_TAG=$IMAGE_TAG_KUBERPULT make -j5 -C ../../charts/kuberpult build-all-docker
  else
    IMAGE_TAG=$IMAGE_TAG_KUBERPULT make -j4 -C ../../charts/kuberpult docker-cd-service docker-manifest-repo-export-service docker-reposerver-service docker-rollout-service
  fi
else
  print 'not building services...'
fi

cd_imagename="${IMAGE_REGISTRY}/kuberpult-cd-service:${IMAGE_TAG_KUBERPULT}"
manifest_repo_export_imagename="${IMAGE_REGISTRY}/kuberpult-manifest-repo-export-service:${IMAGE_TAG_KUBERPULT}"
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

print 'loading kuberpult docker images into kind...'
print "$cd_imagename"
print "$frontend_imagename"
(
  for image in "$cd_imagename" "$manifest_repo_export_imagename" "$rollout_imagename" "$reposerver_imagename" "$frontend_imagename"
  do
    kind load docker-image "${image}" &
  done
  print "waiting to load all kuberpult docker images..."
  wait
)

print 'ensuring that the helm chart is build...'
# it was already build, but we are in another workflow now, so we have to rebuild it
make all

## argoCd

print "starting argoCd..."

helm repo add argo-cd https://argoproj.github.io/argo-helm


reposerver_service=kuberpult-reposerver-service
namespace=default

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

# In kind mode, clean up any ArgoCD installation with a different release name/namespace.
# Helm refuses to adopt CRDs owned by a different release; removing them here lets
# the install below start clean. On GKE the cluster pre-owns ArgoCD, so skip this.
if [ "${ARGO_NAMESPACE}" = "default" ] && helm -n tools status argo-cd > /dev/null 2>&1; then
    print "Found conflicting ArgoCD release 'argo-cd' in namespace 'tools'. Removing it..."
    helm -n tools uninstall argo-cd || true
    kubectl delete crd \
        applications.argoproj.io \
        applicationsets.argoproj.io \
        appprojects.argoproj.io \
        2>/dev/null || true
fi

helm upgrade --install --history-max 1 argocd argo-cd/argo-cd --values argocd-values.yml --version 5.36.0 || exit 1

print applying app...

#waitForDeployment "default" "app.kubernetes.io/name=argocd-repo-server"
waitForDeployment "default" "app.kubernetes.io/name=argocd-server"
portForwardAndWait "default" service/argocd-server 5001 443

# For now, we are only creating development here
# This means argo cd will only handle development, including the rollout-status

# when testing on our gke environment, note that the namespace is different:
#export ARGO_NAMESPACE=tools
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

argocd login localhost:5001 --username admin --password "$argocd_adminpw" --insecure



kubectl create ns development  || echo "already exists"
kubectl create ns development2 || echo "already exists"
kubectl create ns staging      || echo "already exists"
kubectl create ns aa-test      || echo "already exists"


export GIT_NAMESPACE=${GIT_NAMESPACE}
export ARGO_NAMESPACE=${ARGO_NAMESPACE}
export LOCAL_EXECUTION=${LOCAL_EXECUTION}

./install-kuberpult-helm.sh

print 'checking for pods and waiting for portforwarding to be ready...'

kubectl get deployment
kubectl get pods

export FRONTEND_PORT=5002
export CD_GRPC_PORT=5004

#print "running bracket stability integration tests..."
#make kind-test GO_TEST_ARGS="${GO_TEST_ARGS}"

if false; then
  (cd ../../infrastructure/scripts/create-testdata/ ; sh create-environments.sh)

  START=30
  NUM_RELEASES=2
  END=$((START + NUM_RELEASES))

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
fi

echo "done. Kind cluster is up and kuberpult and argoCd are running."
