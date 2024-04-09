#!/usr/bin/env bash
sourcedir="$(dirname $0)"
standard_setup="${FDC_STANDARD_SETUP:-${sourcedir}/../../../../fdc-standard-setup}"
secrets_file="${standard_setup}/secrets/fdc-standard-setup-dev-env-925fe612820f.json"
iap_clientId=$(sops exec-file "${secrets_file}" "jq -r '.client_id' {}")
iap_clientSecret=$(sops exec-file "${secrets_file}" "jq -r '.private_key' {}")
export GIT_NAMESPACE=git
export ARGO_NAMESPACE=default


cat <<VALUES > vals.yaml
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
ssh:
  identity: |
$(gsed -e "s/^/    /" <../../services/cd-service/client)
  known_hosts: |
$(gsed -e "s/^/    /" <../../services/cd-service/known_hosts)
argocd:
  token: "$token"
  server: "https://argocd-server.${ARGO_NAMESPACE}.svc.cluster.local:443"
  insecure: true
  refresh:
    enabled: true
manageArgoApplications:
  enabled: false
  filter: ""
datadogProfiling:
  enabled: false
  apiKey: invalid-3
pgp:
  keyRing: |
$(gsed -e "s/^/    /" <./kuberpult-keyring.gpg)
auth:
  dexAuth:
    enabled: true
    installDex: true
    policy_csv: |
$(for action in CreateLock DeleteLock CreateRelease DeployRelease CreateUndeploy DeployUndeploy CreateEnvironment DeployReleaseTrain; do
        echo "      Developer, $action, development:*, *, allow"
      done)
    clientId: "${iap_clientId}"
    clientSecret: |
$(gsed -e "s/^/      /" <<<"${iap_clientSecret}")
    baseURL: "http://kuberpult-dex-service:5556"
dex:
  outh2:
    grantTypes:
      # ensure grantTypes includes the token-exchange grant (default)
      - foo
  connectors:
    - type: google
      id: google
      name: Google
      config:
        clientID: "${iap_clientId}"
        clientSecret: |
$(gsed -e "s/^/          /" <<<"${iap_clientSecret}")
  config:
    # Set it to a valid URL
    issuer: "http://kuberpult-dex-service:5556"

    # See https://dexidp.io/docs/storage/ for more options
    storage:
      type: memory

    # Enable at least one connector
    # See https://dexidp.io/docs/connectors/ for more options
    enablePasswordDB: true

  staticClients:
    # dex issued tokens are bound to clients.
    # For the token exchange flow, the client id and secret pair must be submitted as the username:password
    # via Basic Authentication.
    - name: My App
      id: my-app
      secret: my-secret
      # We set public to indicate we don't intend to keep the client secret actually secret.
      # https://dexidp.io/docs/custom-scopes-claims-clients/#public-clients
      public: tru
VALUES

# Get helm dependency charts and unzip them
(rm -rf charts && helm dep update && cd charts && for filename in *.tgz; do tar -xf "$filename" && rm -f "$filename"; done;)

echo "Creating tmp.mpl"
helm template ./ --values vals.yaml > tmp.tmpl
helm uninstall kuberpult-local || echo IGNORE
echo "helm install"
helm install --values vals.yaml kuberpult-local ./
echo 'checking for pods and waiting for portforwarding to be ready...'