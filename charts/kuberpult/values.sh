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
    installDex: false
    policy_csv: |
$(for action in CreateLock DeleteLock CreateRelease DeployRelease CreateUndeploy DeployUndeploy CreateEnvironment DeployReleaseTrain; do
        echo "      Developer, $action, development:*, *, allow"
      done)
    clientId: "${iap_clientId}"
    clientSecret: |
$(gsed -e "s/^/      /" <<<"${iap_clientSecret}")
    baseURL: "http://kuberpult-dex-service:5556"

VALUES




cat <<VALUES > dexVals.yaml
connectors:
  - type: google
    id: google
    name: Google
    config:
      clientID: "${iap_clientId}"
      clientSecret: |
$(gsed -e "s/^/          /" <<<"${iap_clientSecret}")
  - name: My Upstream
      type: oidc
      id: my-upstream
      config:
        # The client submitted subject token will be verified against the issuer given here.
        issuer: https://token.example.com
        # Additional scopes in token response, supported list at:
        # https://dexidp.io/docs/custom-scopes-claims-clients/#scopes
        scopes:
          - groups
          - federated:id
        # mapping of fields from the submitted token
        userNameKey: sub
        # Access tokens are generally considered opaque.
        # We check their validity by calling the user info endpoint if it's supported.
        # getUserInfo: true

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
    public: true
 DEPRECATED: use config.yaml.dist and config.dev.yaml examples in the repository root.
 TODO: keep this until all references are updated.

 The base path of dex and the external name of the OpenID Connect service.
 This is the canonical URL that all clients MUST use to refer to dex. If a
 path is provided, dex's HTTP service will listen at a non-root URL.
VALUES


cat <<VALUES > dexVals2.yaml
config:
  issuer: http://127.0.0.1:5556/dex

  # The storage configuration determines where dex stores its state. Supported
  # options include SQL flavors and Kubernetes third party resources.
  #
  # See the documentation (https://dexidp.io/docs/storage/) for further information.
  storage:
    type: sqlite3
    config:
      file: /var/dex/dex.db

  # type: mysql
  # config:
  #   host: localhost
  #   port: 3306
  #   database: dex
  #   user: mysql
  #   password: mysql
  #   ssl:
  #     mode: "false"

  # type: postgres
  # config:
  #   host: localhost
  #   port: 5432
  #   database: dex
  #   user: postgres
  #   password: postgres
  #   ssl:
  #     mode: disable

  # type: etcd
  # config:
  #   endpoints:
  #     - http://localhost:2379
  #   namespace: dex/

  # type: kubernetes
  # config:
  #   kubeConfigFile: $HOME/.kube/config

# Configuration for the HTTP endpoints.
web:
  http: 0.0.0.0:5556
  # Uncomment for HTTPS options.
  # https: 127.0.0.1:5554
  # tlsCert: /etc/dex/tls.crt
  # tlsKey: /etc/dex/tls.key
  # headers:
  #   X-Frame-Options: "DENY"
  #   X-Content-Type-Options: "nosniff"
  #   X-XSS-Protection: "1; mode=block"
  #   Content-Security-Policy: "default-src 'self'"
  #   Strict-Transport-Security: "max-age=31536000; includeSubDomains"


# Configuration for dex appearance
# frontend:
#   issuer: dex
#   logoURL: theme/logo.png
#   dir: web/
#   theme: light

# Configuration for telemetry
telemetry:
  http: 0.0.0.0:5558
  # enableProfiling: true

# Uncomment this block to enable the gRPC API. This values MUST be different
# from the HTTP endpoints.
# grpc:
#   addr: 127.0.0.1:5557
#   tlsCert: examples/grpc-client/server.crt
#   tlsKey: examples/grpc-client/server.key
#   tlsClientCA: examples/grpc-client/ca.crt

# Uncomment this block to enable configuration for the expiration time durations.
# Is possible to specify units using only s, m and h suffixes.
# expiry:
#   deviceRequests: "5m"
#   signingKeys: "6h"
#   idTokens: "24h"
#   refreshTokens:
#     reuseInterval: "3s"
#     validIfNotUsedFor: "2160h" # 90 days
#     absoluteLifetime: "3960h" # 165 days

# Options for controlling the logger.
# logger:
#   level: "debug"
#   format: "text" # can also be "json"

# Default values shown below
# oauth2:
    # grantTypes determines the allowed set of authorization flows.
#   grantTypes:
#     - "authorization_code"
#     - "refresh_token"
#     - "implicit"
#     - "password"
#     - "urn:ietf:params:oauth:grant-type:device_code"
#     - "urn:ietf:params:oauth:grant-type:token-exchange"
    # responseTypes determines the allowed response contents of a successful authorization flow.
    # use ["code", "token", "id_token"] to enable implicit flow for web-only clients.
#   responseTypes: [ "code" ] # also allowed are "token" and "id_token"
    # By default, Dex will ask for approval to share data with application
    # (approval for sharing data from connected IdP to Dex is separate process on IdP)
#   skipApprovalScreen: false
    # If only one authentication method is enabled, the default behavior is to
    # go directly to it. For connected IdPs, this redirects the browser away
    # from application to upstream provider such as the Google login page
#   alwaysShowLoginScreen: false
    # Uncomment the passwordConnector to use a specific connector for password grants
#   passwordConnector: local

# Instead of reading from an external storage, use this list of clients.
#
# If this option isn't chosen clients may be added through the gRPC API.
staticClients:
- id: example-app
  redirectURIs:
  - 'http://127.0.0.1:5555/callback'
  name: 'Example App'
  secret: ZXhhbXBsZS1hcHAtc2VjcmV0
#  - id: example-device-client
#    redirectURIs:
#      - /device/callback
#    name: 'Static Client for Device Flow'
#    public: true
connectors:
- type: mockCallback
  id: mock
  name: Example
# - type: google
#   id: google
#   name: Google
#   config:
#     issuer: https://accounts.google.com
#     # Connector config values starting with a "$" will read from the environment.
#     clientID: $GOOGLE_CLIENT_ID
#     clientSecret: $GOOGLE_CLIENT_SECRET
#     redirectURI: http://127.0.0.1:5556/dex/callback
#     hostedDomains:
#     - $GOOGLE_HOSTED_DOMAIN

# Let dex keep a list of passwords which can be used to login to dex.
enablePasswordDB: true

# A static list of passwords to login the end user. By identifying here, dex
# won't look in its underlying storage for passwords.
#
# If this option isn't chosen users may be added through the gRPC API.
staticPasswords:
- email: "admin@example.com"
  # bcrypt hash of the string "password": $(echo password | htpasswd -BinC 10 admin | cut -d: -f2)
  hash: "$2a$10$2b2cU8CPhOTaGrs1HRQuAueS7JTT5ZHsHSzYiFPm1leZck7Mc8T4W"
  username: "admin"
  userID: "08a8684b-db88-4b73-90a9-3cd1661f5466"
VALUES

cat <<VALUES > dexVals3.yaml
config:
  issuer: https://dex.example.com
  storage:
      type: sqlite3
      config:
          file: /var/dex/dex.db
web:
    http: 0.0.0.0:8001

outh2:
  grantTypes:
    # ensure grantTypes includes the token-exchange grant (default)
    - "urn:ietf:params:oauth:grant-type:token-exchange"

connectors:
  - name: My Upstream
    type: oidc
    id: my-upstream
    config:
      # The client submitted subject token will be verified against the issuer given here.
      issuer: https://token.example.com
      # Additional scopes in token response, supported list at:
      # https://dexidp.io/docs/custom-scopes-claims-clients/#scopes
      scopes:
        - groups
        - federated:id
      # mapping of fields from the submitted token
      userNameKey: sub
      # Access tokens are generally considered opaque.
      # We check their validity by calling the user info endpoint if it's supported.
      # getUserInfo: true

staticClients:
  # dex issued tokens are bound to clients.
  # For the token exchange flow, the client id and secret pair must be submitted as the username:password
  # via Basic Authentication.
  - name: My App
    id: my-app
    secret: my-secret
    # We set public to indicate we don't intend to keep the client secret actually secret.
    # https://dexidp.io/docs/custom-scopes-claims-clients/#public-clients
    public: true
VALUES
# Get helm dependency charts and unzip them
(rm -rf charts && helm dep update && cd charts && for filename in *.tgz; do tar -xf "$filename" && rm -f "$filename"; done;)

echo "Creating tmp.mpl"
helm template ./ --values vals.yaml > tmp.tmpl
helm uninstall kuberpult-local || echo IGNORE
echo "helm install"
cat vals.yaml
helm install --values vals.yaml kuberpult-local ./
#echo 'checking for pods and waiting for portforwarding to be ready...'
#
helm uninstall dex-local || echo IGNORE
helm install --values dexVals3.yaml dex-local ./charts/dex
