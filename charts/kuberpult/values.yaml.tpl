# Default values for ..
# This is a YAML-formatted file.
# Declare variables to be passed into your templates.

git:
  url:  # git@github.com/.../...
  branch: "master"
  sourceRepoUrl: ""
  author:
    name: local.user@example.com
    email: defaultUser

hub: europe-west3-docker.pkg.dev/fdc-public-docker-registry/kuberpult

log:
  # Possible values are "gcp" for a gcp-optimized format and "default" for json
  format: ""
  # Other possible values are "DEBUG", "INFO", "ERROR"
  level: "WARN"
cd:
  image: kuberpult-cd-service
# In MOST cases, do NOT overwrite the "tag".
# In general, kuberpult only guarantees that running with the same version of frontend and cd service will work.
# For testing purposes, we allow to overwrite the tags individually, to test an old frontend service with a new cd service.
  tag: "$VERSION"
  backendConfig:
    create: false   # Add backend config for health checks on GKE only
    timeoutSec: 300  # 30sec is the default on gcp loadbalancers, however kuberpult needs more with parallel requests. It is the time how long the loadbalancer waits for kuberpult to finish calls to the rest endpoint "release"
  resources:
    limits:
      cpu: 2
      memory: 3Gi
    requests:
      cpu: 2
      memory: 3Gi
  enableSqlite: true
frontend:
  image: kuberpult-frontend-service
# In MOST cases, do NOT overwrite the "tag".
# In general, kuberpult only guarantees that running with the same version of frontend and cd service will work.
# For testing purposes, we allow to overwrite the tags individually, to test an old frontend service with a new cd service.
  tag: "$VERSION"
  resources:
    limits:
      cpu: 500m
      memory: 250Mi
    requests:
      cpu: 500m
      memory: 250Mi
rollout:
  enabled: false
  image: kuberpult-rollout-service
# In MOST cases, do NOT overwrite the "tag".
# In general, kuberpult only guarantees that running with the same version of frontend and cd service will work.
# For testing purposes, we allow to overwrite the tags individually, to test an old frontend service with a new cd service.
  tag: "$VERSION"
  resources:
    limits:
      cpu: 500m
      memory: 250Mi
    requests:
      cpu: 500m
      memory: 250Mi

ingress:
  create: true
  annotations: {}
  domainName: null
  iap:
    enabled: false
    secretName: null
  tls:
    host: null
    secretName: kuberpult-tls-secret
ssh:
  identity: |
    -----BEGIN OPENSSH PRIVATE KEY-----
    -----END OPENSSH PRIVATE KEY-----
  known_hosts: |
    github.com ecdsa-sha2-nistp256 AAAAE2VjZHNhLXNoYTItbmlzdHAyNTYAAAAIbmlzdHAyNTYAAABBBEmKSENjQEezOmxkZMy7opKgwFB9nkt5YRrYMjNuG5N87uRgg6CLrbo5wAdT/y6v0mKV0U2w0WZ2YB/++Tpockg=
pgp:
  keyRing: null

argocd:
  baseUrl: ""
  # The token is generated by adding a user in argocd with apiKey permssions and generating a token.
  # 1. Add an entry to the configmap argocd-cm data with key "accounts.kuberpult" and value "apiKey"
  # 2. Run `argocd account generate-token --account kuberpult` and put the result here
  # 3. Grant kuberpult the necessary rights by adding these lines to the argocd-rbac-cm config map:
  #
  #  policy.csv: |
  #    p, role:kuberpult, applications, get, */*, allow
  #    g, kuberpult, role:kuberpult
  #
  token: ""
  # Argocd server url. If argocd is running in the same cluster, use the service name of the api server.
  server: ""
  # Disables tls verification. This is useful when running in the same cluster as argocd and using a self-signed certificate.
  insecure: false

datadogTracing:
  enabled: false
  debugging: false
  environment: "shared"

dogstatsdMetrics:
  enabled: false
  #  dogstatsD listens on port udp:8125 by default.
  #  https://docs.datadoghq.com/developers/dogstatsd/?tab=hostagent#agent
  #  datadog.dogstatsd.socketPath -- Path to the DogStatsD socket
  address: unix:///var/run/datadog/dsd.socket
  # datadog.dogstatsd.hostSocketPath -- Host path to the DogStatsD socket
  hostSocketPath: /var/run/datadog

imagePullSecrets: []

gke:
  # The backend service id and project number are used to verify IAP tokens.
  #
  # The backend service id can only be obtained _after_ everything is installed.
  # Use this bash script to obtain it (after login to gcloud and select the correct project):
  #
  # ```
  # gcloud compute backend-services describe --global $(gcloud compute backend-services list | grep "kuberpult-frontend-service-80" | cut -d" " -f1) | yq .id
  # ```
  backend_service_id: ""
  #
  # Use this bash script to obtain the project number:
  #
  # ```
  # gcloud projects list --filter="$(gcloud config get-value project)" --format="value(PROJECT_NUMBER)"
  # ```
  project_number: ""

environment_configs:
  bootstrap_mode: false
  # environment_configs_json: |
  #   {
  #     "production": {
  #       "upstream": {
  #           "latest": true
  #        },
  #        "argocd" :{}
  #     }
  #   }
  environment_configs_json: null

auth:
  azureAuth:
    enabled: false
    cloudInstance: "https://login.microsoftonline.com/"
    clientId: ""
    tenantId: ""

dex:
  enabled: false
  # Defines the rbac policy when using Dex.
  # The permissions are added using the following format (<ROLE>, <ACTION>, <ENVIRONMENT_GROUP>:<ENVIRONMENT>, <APPLICATION>, allow).
  #
  # Available actions are: CreateLock, DeleteLock, CreateRelease, DeployRelease, CreateUndeploy, DeployUndeploy, CreateEnvironment, CreateEnvironmentApplication and DeployReleaseTrain.
  # The actions CreateUndeploy, DeployUndeploy and CreateEnvironmentApplication are environment independent meaning that the environment specified on the permission
  # needs to follow the following format <ENVIRONMENT_GROUP>:*, otherwise an error will be thrown.
  #
  # Example permission: Developer, CreateLock, development:development, *, allow
  # If no group is configured for an environment, the environment group name is the same as the environment name, here "development".
  # The policy will be available on the kuberpult-rbac config map.
  policy_csv: ""
  clientId: ""
  clientSecret: ""
  baseURL: ""
  # List of scopes to validate the token. Please add them as comma separated values.
  scopes: ""
