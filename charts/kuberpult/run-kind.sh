#!/usr/bin/env bash

set -eu
set -o pipefail

# This script assumes that the docker images have already been built.
# To run/debug/develop this locally, you probably want to run like this:
# rm -rf ./manifests/; make clean; LOCAL_EXECUTION=true ./run-kind.sh

cd "$(dirname "$0")"


# prefix every call to "echo" with the name of the script:
function print() {
  /bin/echo "$0:" "$@"
}

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

function waitForDeployment() {
  ns="$1"
  label="$2"
  print "waitForDeployment: $ns/$label"
  sleep 10
  until kubectl wait --for=condition=ready pod -n "$ns" -l "$label" --timeout=30s
  do
    sleep 4
    print "logs:"
    kubectl -n "$ns" logs -l "$label" || echo "could not get logs for $label"
    print "describe pod:"
    kubectl -n "$ns" describe pod -l "$label"
#    print "describe pod:"
#    kubectl -n "$ns" describe pod -l app=kuberpult-cd-service || echo "could not describe pod"
    print ...
  done
}

function portForwardAndWait() {
  ns="$1"
  deployment="$2"
  portHere="$3"
  portThere="$4"
  ports="$portHere:$portThere"
  print "portForwardAndWait for $ns/$deployment $ports"
  kubectl -n "$ns" port-forward "$deployment" "$ports" &
  print "portForwardAndWait: waiting until the port forward works..."
  sleep 10
  until nc -vz localhost "$portHere"
  do
    sleep 3
    print "logs:"
    kubectl -n "$ns" logs "$deployment"
    print "describe deployment:"
    kubectl -n "$ns" describe "$deployment"
    print "describe pod:"
    kubectl -n "$ns" describe pod -l app=kuberpult-cd-service || echo "could not describe pod"
    print ...
  done
}

GPG="gpg --keyring trustedkeys-kuberpult.gpg"
gpgFile=~/.gnupg/trustedkeys-kuberpult.gpg
if test -f "$gpgFile"
then
  echo warning: file already exists: "$gpgFile"
  if "$LOCAL_EXECUTION"
  then
    echo "is it ok to delete the file? Press enter twice to delete"
    # shellcheck disable=SC2162
    read
    # shellcheck disable=SC2162
    read
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
cp -r ../../../infrastructure/scripts/create-testdata/testdata_template/environments .
git add environments
GIT_AUTHOR_NAME='Initial Kuberpult Commiter' GIT_COMMITTER_NAME='Initial Kuberpult Commiter' GIT_AUTHOR_EMAIL='team.sre.permanent+kuberpult-initial-commiter@freiheit.com'  GIT_COMMITTER_EMAIL='team.sre.permanent+kuberpult-initial-commiter@freiheit.com' git commit -m "add initial environments from template"
print "pushing environments to manifest repo..."
GIT_SSH_COMMAND='ssh -o UserKnownHostsFile=emptyfile -o StrictHostKeyChecking=no -i ../../../services/cd-service/client' git checkout -B main
GIT_SSH_COMMAND='ssh -o UserKnownHostsFile=emptyfile -o StrictHostKeyChecking=no -i ../../../services/cd-service/client' git push -f origin main
cd -


export IMAGE_REGISTRY=europe-west3-docker.pkg.dev/fdc-public-docker-registry/kuberpult

if "$LOCAL_EXECUTION"
then
  print 'building services...'
  make -C ../.. all-services
else
  print 'not building services...'
fi

print version...
VERSION=$(make --no-print-directory -C ../../ version)
print "version is ${VERSION}"
IMAGE_TAG_KUBERPULT=${IMAGE_TAG_KUBERPULT:-$VERSION}
print "IMAGE_TAG_KUBERPULT is now ${IMAGE_TAG_KUBERPULT}"

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


helm uninstall argocd || echo "did not uninstall argo"
cat <<YAML > argocd-values.yml
configs:
  ssh:
    knownHosts: |
$(sed -e "s/^/        /" <../../services/cd-service/known_hosts)
  cm:
    accounts.kuberpult: apiKey
    timeout.reconciliation: 0s
  params:
    controller.repo.server.plaintext: "true"
    server.repo.server.plaintext: "true"
    repo.server: kuberpult-reposerver-service:8443
  rbac:
    policy.csv: |
      p, role:kuberpult, applications, get, */*, allow
      p, role:kuberpult, applications, create, */*, allow
      p, role:kuberpult, applications, update, */*, allow
      p, role:kuberpult, applications, sync, */*, allow
      p, role:kuberpult, applications, delete, */*, allow
      g, kuberpult, role:kuberpult

YAML
helm install argocd argo-cd/argo-cd --values argocd-values.yml --version 5.36.0

print applying app...

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
kind: Application
metadata:
  name: root
  namespace: ${ARGO_NAMESPACE}
spec:
  destination:
    namespace: ${ARGO_NAMESPACE}
    server: https://kubernetes.default.svc
  project: test-env
  source:
    path: argocd/v1alpha1
    repoURL: ssh://git@server.${GIT_NAMESPACE}.svc.cluster.local/git/repos/manifests
    targetRevision: HEAD
  syncPolicy:
    automated: {}
EOF

waitForDeployment "default" "app.kubernetes.io/name=argocd-server"
portForwardAndWait "default" service/argocd-server 8080 443
print "admin password:"
argocd_adminpw=$(kubectl -n default get secret argocd-initial-admin-secret -o jsonpath="{.data.password}" | base64 -d)
echo "$argocd_adminpw"
echo "$argocd_adminpw" > argocd_adminpw.txt

argocd login --port-forward --username admin --password "$argocd_adminpw"

token=$(argocd account generate-token --port-forward --account kuberpult)

echo "argocd token: $token"


kubectl create ns development
kubectl create ns development2
kubectl create ns staging


export GIT_NAMESPACE=${GIT_NAMESPACE}
export ARGO_NAMESPACE=${ARGO_NAMESPACE}
export LOCAL_EXECUTION=${LOCAL_EXECUTION}
export TOKEN=${token}

./install-kuberpult-helm.sh

print 'checking for pods and waiting for portforwarding to be ready...'

kubectl get deployment
kubectl get pods

print "port forwarding to cd service..."
waitForDeployment "default" "app=kuberpult-cd-service"
portForwardAndWait "default" deployment/kuberpult-cd-service 8082 8080

waitForDeployment "default" "app=kuberpult-frontend-service"
portForwardAndWait "default" "deployment/kuberpult-frontend-service" "8081" "8081"
print "connection to frontend service successful"
waitForDeployment "default" "app=kuberpult-rollout-service"
print "connection to rollout service successful"

kubectl get deployment
kubectl get pods

(cd ../../infrastructure/scripts/create-testdata/ ; sh create-environments.sh)

for v in $(seq 1 3)
do
   RELEASE_VERSION=$v ../../infrastructure/scripts/create-testdata/create-release.sh echo;
done


if "$LOCAL_EXECUTION"
then
  echo "hit ctrl+c to stop"
  read -r -d '' _ </dev/tty
else
  echo "done. Kind cluster is up and kuberpult and argoCd are running."
fi
