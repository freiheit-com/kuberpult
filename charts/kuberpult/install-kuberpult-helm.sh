#!/usr/bin/env bash

# shellcheck source=/dev/null
source "$(dirname "$0")/lib.sh"

# prefix every call to "echo" with the name of the script:
function print() {
  /bin/echo "$0:" "$@"
}

print 'installing kuberpult helm chart...'

portForwardAndWait "default" service/argocd-server 5001 443
token=$(argocd account generate-token --server localhost:5001 --account kuberpult --insecure)

echo "argocd token: $token" # this is only ok because this script is only used locally for a temporary cluster. Never do this on production.


GIT_NAMESPACE=${GIT_NAMESPACE:-git}
ARGO_NAMESPACE=${ARGO_NAMESPACE:-default}
VERSION=$(git describe --always --long --tags || echo 0.0.1)

set -eu
set -o pipefail

cat <<VALUES > vals.yaml
auth:
  api:
    enableDespiteNoAuth: true
db:
  location: postgres
  authProxyPort: 5432
  dbName: kuberpult
  dbUser: postgres
  dbPassword: mypassword
  k8sServiceAccountName: default
  sslMode: disable
cd:
  resources:
    limits:
      memory: 200Mi
      cpu: 0.05
    requests:
      memory: 200Mi
      cpu: 0.05
frontend:
  resources:
    limits:
      memory: 200Mi
      cpu: 0.05
    requests:
      memory: 200Mi
      cpu: 0.05
rollout:
  experimentalBrackets:
    enabled: true
    clusters:
      development: false
      staging: false
      aa-aa-test-dev-1: false
  enabled: true
  grpcMaxRecvMsgSize: 4
  resources:
    limits:
      memory: 200Mi
      cpu: 0.05
    requests:
      memory: 200Mi
      cpu: 0.05
  persistArgoEvents: true
  argoEventsBatchSize: 1
reposerver:
  enabled: true
  grpcMaxRecvMsgSize: 4
  resources:
    limits:
      memory: 200Mi
      cpu: 0.05
    requests:
      memory: 200Mi
      cpu: 0.05
manifestRepoExport:
  enabled: false
  eslProcessingIdleTimeSeconds: 10
  resources:
    limits:
      memory: 200Mi
      cpu: 0.05
    requests:
      memory: 200Mi
      cpu: 0.05
  experimentalRolloutWithManifest:
    enabled: true
    argoProjectNames:
      environments:
        staging: staging-proj-override666
      aaEnvironments:
        aa-aa-test-dev-1: aa-proj-override666
ingress:
  domainName: kuberpult.example.com
log:
  level: INFO
git:
  url: "ssh://git@server.${GIT_NAMESPACE}.svc.cluster.local/git/repos/manifests"
  sourceRepoUrl: "https://github.com/freiheit-com/kuberpult/tree/{branch}/{dir}"
  branch: "main"
  networkTimeout: 1s
  enableWritingCommitData: false
ssh:
  identity: |
$(sed -e "s/^/    /" <../../services/cd-service/client)
  known_hosts: |
$(sed -e "s/^/    /" <../../services/cd-service/known_hosts)
argocd:
  token: "$token"
  server: "https://argocd-server.${ARGO_NAMESPACE}.svc.cluster.local:443"
  insecure: true
  refresh:
    enabled: true
manageArgoApplications:
  enabled: true
  filter: "*"
datadogProfiling:
  enabled: false
  apiKey: invalid-3
pgp:
  keyRing: |
$(sed -e "s/^/    /" <./kuberpult-keyring.gpg)
VALUES

# Get helm dependency charts and unzip them
(rm -rf charts && helm dep update && cd charts && for filename in *.tgz; do echo "$filename"; tar -xf "$filename" && rm -f "$filename"; done;)

make release-tag

#helm uninstall kuberpult-local || print kuberpult was not installed
helm upgrade --install --history-max 1 --values vals.yaml kuberpult-local kuberpult-"$VERSION".tgz || exit 1


kubectl get deployment
kubectl get pods

waitForDeployment "default" "app=kuberpult-frontend-service"
print "port forwarding to cd service..."
waitForDeployment "default" "app=kuberpult-cd-service"
portForwardAndWait "default" deployment/kuberpult-cd-service 5003 8080
portForwardAndWait "default" deployment/kuberpult-cd-service 5004 8443

