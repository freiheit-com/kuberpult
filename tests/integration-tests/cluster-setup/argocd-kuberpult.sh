#!/usr/bin/env bash

set -eu
set -o pipefail

# prefix every call to "echo" with the name of the script:
function print() {
  /bin/echo "$0:" "$@"
}

function waitForDeployment() {
  ns="$1"
  label="$2"
  print "waitForDeployment: $ns/$label"
  until kubectl wait --for=condition=ready pod -n "$ns" -l "$label" --timeout=30s
  do
    sleep 4s
    print "logs:"
    kubectl -n "$ns" logs -l "$label" || echo "could not get logs for $label"
    print "describe pod:"
    kubectl -n "$ns" describe pod -l "$label"
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
  until nc -vz localhost "$portHere"
  do
    sleep 1s
  done
}

print "setting up manifest repo"
waitForDeployment "git" "app.kubernetes.io/name=server"
portForwardAndWait "git" "deployment/server" "$SSH_HOST_PORT" "22"

git clone ssh://git@localhost:$SSH_HOST_PORT/git/repos/manifests

cp -r environments manifests/

cd manifests
git add environments
git commit -m "add initial environments from template"
print "pushing environments to manifest repo..."
git push origin master

cd -

## argoCd
print "starting argoCd..."

helm repo add argo-cd https://argoproj.github.io/argo-helm

cat <<YAML > argocd-values.yml
configs:
  ssh:
    knownHosts: |
$(sed -e "s/^/        /" </kp/known_hosts)
  cm:
    accounts.kuberpult: apiKey
    timeout.reconciliation: 0s
  params:
    controller.repo.server.plaintext: "true"
    server.repo.server.plaintext: "true"
    repo.server: kuberpult-cd-service:8443
  rbac:
    policy.csv: |
      p, role:kuberpult, applications, get, */*, allow
      g, kuberpult, role:kuberpult

YAML
helm install argocd argo-cd/argo-cd --values argocd-values.yml --version 5.36.0

print applying app...

kubectl apply -f - <<EOF
apiVersion: argoproj.io/v1alpha1
kind: AppProject
metadata:
  name: test-env
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

argocd_adminpw=$(kubectl -n default get secret argocd-initial-admin-secret -o jsonpath="{.data.password}" | base64 -d)
argocd login --port-forward --username admin --password "$argocd_adminpw"
token=$(argocd account generate-token --port-forward --account kuberpult)

kubectl create ns development
kubectl create ns development2
kubectl create ns staging

## kuberpult
print 'installing kuberpult helm chart...'

cat <<VALUES > vals.yaml
cd:
  db:
    dbOption: sqlite
    location: /sqlite
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
ssh:
  identity: |
$(sed -e "s/^/    /" </kp/client)
  known_hosts: |
$(sed -e "s/^/    /" </kp/known_hosts)
argocd:
  token: "$token"
  server: "https://argocd-server.${ARGO_NAMESPACE}.svc.cluster.local:443"
  insecure: true
  refresh:
    enabled: true
pgp:
  keyRing: |
$(sed -e "s/^/    /" </kp/kuberpult-keyring.gpg)
VALUES

# Get helm dependency charts and unzip them
(rm -rf charts && helm dep update && cd charts && for filename in *.tgz; do tar -xf "$filename" && rm -f "$filename"; done;)
helm template ./ --values vals.yaml --set generateMigrations=true > tmp.tmpl
helm install --values vals.yaml kuberpult-local ./
print 'checking for pods and waiting for portforwarding to be ready...'
rm -rf migrations
kubectl get deployment
kubectl get pods

print "port forwarding to cd service..."
waitForDeployment "default" "app=kuberpult-cd-service"
portForwardAndWait "default" deployment/kuberpult-cd-service 8082 8080

waitForDeployment "default" "app=kuberpult-frontend-service"
portForwardAndWait "default" "deployment/kuberpult-frontend-service" "8081" "8081"
print "connection to frontend service successful"

kubectl get deployment
kubectl get pods

for i in $(seq 1 3)
do
   /kp/create-release.sh echo;
done

echo "Done. Kuberpult and argoCd are running."
