# Default values for ..
# This is a YAML-formatted file.
# Declare variables to be passed into your templates.

git:
  url:  # git@github.com/.../...
  branch: "master"

hub: europe-west3-docker.pkg.dev/fdc-public-docker-registry/kuberpult
tag: "$VERSION"

log:
  # Possible values are "gcp" for a gcp-optimized format and "default" for json
  format: ""
  # Other possible values are "DEBUG", "INFO", "ERROR"
  level: "WARN"
cd:
  image: kuberpult-cd-service
  backendConfig:
    create: false   # Add backend config for health checks on GKE only
    timeoutSec: 30  # 30 is the default at least on gcp. It is the time how long the loadbalancer waits for kuberpult to finish calls to the rest endpoint "release"
  resources:
    limits:
      cpu: 2
      memory: 3Gi
    requests:
      cpu: 2
      memory: 3Gi
frontend:
  image: kuberpult-frontend-service
  resources:
    limits:
      cpu: 500m
      memory: 100Mi
    requests:
      cpu: 500m
      memory: 100Mi
ingress:
  annotations: {}
  domainName: null
  exposeReleaseEndpoint: false
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
  enabled: false
  user: admin
  host: argo-cd-argocd-server

datadogTracing:
  enabled: false
  debugging: false

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
  backend_service_id: ""
  project_number: ""
