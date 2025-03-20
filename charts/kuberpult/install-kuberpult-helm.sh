#!/usr/bin/env bash

# prefix every call to "echo" with the name of the script:
function print() {
  /bin/echo "$0:" "$@"
}

print 'installing kuberpult helm chart...'

LOCAL_EXECUTION=${LOCAL_EXECUTION:-false}
GIT_NAMESPACE=${GIT_NAMESPACE:-git}
ARGO_NAMESPACE=${ARGO_NAMESPACE:-default}
token=${TOKEN:-invalid-i-dont-care}
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
  dbOption: postgreSQL
  writeEslTableOnly: false
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
manifestRepoExport:
  eslProcessingIdleTimeSeconds: 15
  resources:
    limits:
      memory: 200Mi
      cpu: 0.05
    requests:
      memory: 200Mi
      cpu: 0.05
ingress:
  domainName: kuberpult.example.com
log:
  level: INFO
git:
  url: "ssh://git@server.${GIT_NAMESPACE}.svc.cluster.local/git/repos/manifests"
  sourceRepoUrl: "https://github.com/freiheit-com/kuberpult/tree/{branch}/{dir}"
  branch: "main"
  networkTimeout: 1s
  enableWritingCommitData: true
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

earthly +chart-tarball


helm uninstall kuberpult-local || print kuberpult was not installed
helm install --values vals.yaml kuberpult-local kuberpult-"$VERSION".tgz
