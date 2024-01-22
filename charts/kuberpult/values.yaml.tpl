# Default values for ..
# This is a YAML-formatted file.
# Declare variables to be passed into your templates.

git:
  # The git url of the manifest repository (git protocol)
  url:  # git@github.com/.../...
  webUrl:  # only necessary for webhooks to argoCd, e.g. https://github.com/freiheit-com/kuberpult

  # The branch to be use in the manifest repository
  branch: "master"

  # If this is set, kuberpult will render a link to apps in the manifest repository (not the source repo).
  # Use `{dir}` and `{branch}` to automatically replace with proper values
  # Example for GitHub: https://github.com/freiheit-com/kuberpult/tree/{branch}/{dir}
  # Example for BitBucket: http://bitbucket.com/projects/projectName/repos/repoName/browse/{dir}/?at=refs%2Fheads%2F{branch}
  # Example for Azure: https://dev.azure.com/projectName/_git/repoName?path=/{dir}&version=GB{branch}&_a=contents
  manifestRepoUrl: ""

  # If this is set, kuberpult will render a link to the source code of your services (not the manifest repository).
  # Use `{branch}` and `{commit}` to automatically replace with proper values
  # Example for GitHub: https://github.com/freiheit-com/kuberpult/commit/{commit}
  # Example for BitBucket: https://bitbucket.com/path/to/repo/commits/{commit}
  # Example for Azure: https://dev.azure.com/path/to/repo/commit/{commit}?refName=refs%2Fheads%2F{branch}
  sourceRepoUrl: ""

  # The git author is what kuberpult writes to the manifest repository.
  # The git committer cannot be configured. It will always be "kuberpult".
  author:
    name: local.user@example.com
    email: defaultUser

  # Timeout used for network operations
  networkTimeout: 1m

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
  probes:
    liveness:
      initialDelaySeconds: 5
    readiness:
      initialDelaySeconds: 5
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
# Limit for the wait time for resources that support waiting on conditions ( e.g. rollout-status ).
# This MUST be lower than the combined timeouts of ALL http proxies in use.
  maxWaitDuration: 10m
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
  # annotations given here will take precedence over the defaults defined in _helpers.tpl
  podAnnotations: {}

ingress:
  # The simplest setup involves an ingress, to make kuberpult available outside the cluster.
  # set to false, if you want use your own ingress:
  create: true
  annotations:
    nginx.ingress.kubernetes.io/proxy-read-timeout: 300
  domainName: null
  # note that IAP is a GCP specific feature. On GCP we recommend to enable it.
  iap:
    enabled: false
    secretName: null
  tls:
    host: null
    secretName: kuberpult-tls-secret
ssh:
  # This section is necessary to checkout the manifest repo from git. Only ssh is supported (no https).
  identity: |
    -----BEGIN OPENSSH PRIVATE KEY-----
    -----END OPENSSH PRIVATE KEY-----
  known_hosts: |
    github.com ecdsa-sha2-nistp256 AAAAE2VjZHNhLXNoYTItbmlzdHAyNTYAAAAIbmlzdHAyNTYAAABBBEmKSENjQEezOmxkZMy7opKgwFB9nkt5YRrYMjNuG5N87uRgg6CLrbo5wAdT/y6v0mKV0U2w0WZ2YB/++Tpockg=
pgp:
  # The pgp keyring is used as an authentication measure for kuberpult rest endpoints that are publicly available.
  # If you do not use IAP, it is highly recommended to enable this.
  keyRing: null

argocd:
  # The base url is used to generate links to argocd in the UI. Kuberpult never uses this to talk to argocd.
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
  # The argocd server url is used by kuberpult to reach out to argocd. If argocd is running in the same cluster, use the service name of the api server.
  # Also must include the protocol and port e.g. http://argocd-server.argocd.svc.cluster.local:80 or https://argocd.example.com:443
  server: ""
  # Disables tls verification. This is useful when running in the same cluster as argocd and using a self-signed certificate.
  insecure: false
  # Enable sending webhooks to argocd
  sendWebhooks: false

  refresh:
    # Enable sending refresh requests to argocd
    enabled: false
    # Send up to that many parallel refresh requests to argocd.
    # The number is determined by the power of the deployed argocd.
    concurrency: 50

datadogTracing:
  enabled: false
  debugging: false
  environment: "shared"

dogstatsdMetrics:
  # send metrics:
  enabled: false

  # sends additional events for each deployments:
  # dogstatsdMetrics.enabled must be true for this to have an effect.
  eventsEnabled: false

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
  dexAuth:
    enabled: false
    # Indicates if dex is to be installed. If you want to use your own Dex instance do not enable this flag.
    installDex: false
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

# The Dex configuration values. For more information please check the Dex repository https://github.com/dexidp/dex
dex:
  # List of environment variables to be added to the dex service pod.
  # Example, if you want your DEX service to have access to to the OAUTH_CLIENT_ID, you can specify
  # it the following way: 
  # 
  #  - name: OAUTH_CLIENT_ID
  #    valueFrom:
  #      secretKeyRef:
  #      name: kuberpult-oauth-client-id
  #        key: kuberpult-oauth-client-id
  envVars: []
  # The configuration of the OAUTH provider. 
  # For more information on the connectors to use see https://dexidp.io/docs/connectors/
  # Here is an example on how to connect with Google connector:
  #
  #     connectors:
  #     - type: google
  #     id: google
  #     name: Google
  #     config:
  #       clientID: $GOOGLE_CLIENT_ID
  #       clientSecret: $GOOGLE_CLIENT_SECRET
  #       redirectURI: http://127.0.0.1:5556/callback
  config: {}
# Configuration for revolution dora metrics. If you are not using revolution you can safely ignore this.
revolution:
  dora:
    enabled: false
    # The default url in revolution is https://revolution.dev/api/dora/kuberpult?companyID=myCompany&productID=myProductID&projectID=myProductId
    url: ""
    # The token can be obtained from revolution.
    token: ""
    # Maximum number of requests send in parallel.
    concurrency: 20

# Whether the rollout service should self-manage applications
manageArgoApplications:
  enabled: false
  # List of teams that should be self managed by the rollout service.
  filter: []
