package kuberpult_chart_test

import (
	"fmt"
	"github.com/google/go-cmp/cmp"
	"math/rand/v2"
	"os"
	"os/exec"
	"strconv"
	"testing"
)

var tpl_dd_disabled_values = `# This file is part of kuberpult.

# Kuberpult is free software: you can redistribute it and/or modify
# it under the terms of the Expat(MIT) License as published by
# the Free Software Foundation.

# Kuberpult is distributed in the hope that it will be useful,
# but WITHOUT ANY WARRANTY; without even the implied warranty of
# MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
# MIT License for more details.

# You should have received a copy of the MIT License
# along with kuberpult. If not, see <https://directory.fsf.org/wiki/License:Expat>.

# Copyright 2023 freiheit.com
# Default values for ..
# This is a YAML-formatted file.
# Declare variables to be passed into your templates.

git:
  # The git url of the manifest repository (git protocol)
  url: git@github.com/.../...
  webUrl:  # only necessary for webhooks to argoCd, e.g. https://github.com/freiheit-com/kuberpult

  # The branch to be use in the manifest repository
  branch: "master"

  # If this is set, kuberpult will render a link to apps in the manifest repository (not the source repo).

  # Example for GitHub: https://github.com/freiheit-com/kuberpult/tree/{branch}/{dir}
  # Example for BitBucket: http://bitbucket.com/projects/projectName/repos/repoName/browse/{dir}/?at=refs%2Fheads%2F{branch}
  # Example for Azure: https://dev.azure.com/projectName/_git/repoName?path=/{dir}&version=GB{branch}&_a=contents
  manifestRepoUrl: ""

  # If this is set, kuberpult will render a link to the source code of your services (not the manifest repository).

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


  enableWritingCommitData: false

  # Kuberpult tries to reduce the number of pushes and can bundle concurrent commits into a single push.
  # This can reduce the time it takes to process requests ariving at the same time and improve throughput.
  # The correct number largely depends on the performance of the git host and repository size. For small to medium sized deployments the default is good.
  # We recommend values between 1 and 20.
  maximumCommitsPerPush: 5

hub: europe-west3-docker.pkg.dev/fdc-public-docker-registry/kuberpult

log:
  # Possible values are "gcp" for a gcp-optimized format and "default" for json
  format: ""
  # Other possible values are "DEBUG", "INFO", "ERROR"
  level: "WARN"
cd:
  image: kuberpult-cd-service
  backendConfig:
    create: false   # Add backend config for health checks on GKE only
    timeoutSec: 300  # 30sec is the default on gcp loadbalancers, however kuberpult needs more with parallel requests. It is the time how long the loadbalancer waits for kuberpult to finish calls to the rest endpoint "release"
    queueSize: 5
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
      periodSeconds: 10
      successThreshold: 1
      timeoutSeconds: 5
      failureThreshold: 10
      initialDelaySeconds: 5
    readiness:
      periodSeconds: 10
      successThreshold: 1
      timeoutSeconds: 5
      failureThreshold: 10
      initialDelaySeconds: 5
frontend:
  image: kuberpult-frontend-service
# Annotations given here will be added to kuberpult-frontend-service annotations.
# See frontend-service.yaml for automatically added annotations.
  service:
    annotations: {}
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
  batchClient:
    # This value needs to be higher than the network timeout for git
    timeout: 2m
rollout:
  enabled: false
  image: kuberpult-rollout-service
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
  create: false
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

  generateFiles: true

datadogTracing:
  enabled: false
  debugging: false
  environment: "shared"

datadogProfiling:

  enabled: false
  # In order to use the datadog profile, you must provide a datadog api key:
  apiKey: ""

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

  # Alternatively, you can set the name of the backend service as regex, and kuberpult will try to get the id of the first matching backend service using google compute SDK.
  # Only one of backend_service_id and backend_service_name should be set. Setting both is not supported and lead to an error.
  backend_service_id: ""
  backend_service_name: ""
  #
  # Use this bash script to obtain the project number:
  #
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
  api:
    # If you do not have Googles IAP enabled, but still want to use the API, be aware that it is publicly available,
    # so you would need a protection outside of kuberpult (e.g. a VPN).
    enableDespiteNoAuth: false

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

`
var tpl_dd_disabled_expected_output = `---
# Source: kuberpult/templates/cd-service.yaml
apiVersion: v1
kind: Secret
metadata:
  name: kuberpult-ssh
data:
  identity: "LS0tLS1CRUdJTiBPUEVOU1NIIFBSSVZBVEUgS0VZLS0tLS0KLS0tLS1FTkQgT1BFTlNTSCBQUklWQVRFIEtFWS0tLS0tCg=="
  ssh_known_hosts: "Z2l0aHViLmNvbSBlY2RzYS1zaGEyLW5pc3RwMjU2IEFBQUFFMlZqWkhOaExYTm9ZVEl0Ym1semRIQXlOVFlBQUFBSWJtbHpkSEF5TlRZQUFBQkJCRW1LU0VOalFFZXpPbXhrWk15N29wS2d3RkI5bmt0NVlScllNak51RzVOODd1UmdnNkNMcmJvNXdBZFQveTZ2MG1LVjBVMncwV1oyWUIvKytUcG9ja2c9Cg=="
---
# Source: kuberpult/templates/cd-service.yaml
apiVersion: v1
kind: Service
metadata:
  name: kuberpult-cd-service
spec:
  ports:
  - name: http
    port: 80
    targetPort: http
  - name: grpc
    port: 8443
    targetPort: grpc
  selector:
    app: kuberpult-cd-service
  type: NodePort
---
# Source: kuberpult/templates/frontend-service.yaml
apiVersion: v1
kind: Service
metadata:
  name: kuberpult-frontend-service
  annotations:
spec:
  ports:
  - name: http
    port: 80
    targetPort: http
  selector:
    app: kuberpult-frontend-service
  type: NodePort
---
# Source: kuberpult/templates/cd-service.yaml
# This file is part of kuberpult.

# Kuberpult is free software: you can redistribute it and/or modify
# it under the terms of the Expat(MIT) License as published by
# the Free Software Foundation.

# Kuberpult is distributed in the hope that it will be useful,
# but WITHOUT ANY WARRANTY; without even the implied warranty of
# MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
# MIT License for more details.

# You should have received a copy of the MIT License
# along with kuberpult. If not, see <https://directory.fsf.org/wiki/License:Expat>.

# Copyright 2023 freiheit.com
# This file is part of kuberpult.

# Kuberpult is free software: you can redistribute it and/or modify
# it under the terms of the Expat(MIT) License as published by
# the Free Software Foundation.

# Kuberpult is distributed in the hope that it will be useful,
# but WITHOUT ANY WARRANTY; without even the implied warranty of
# MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
# MIT License for more details.

# You should have received a copy of the MIT License
# along with kuberpult. If not, see <https://directory.fsf.org/wiki/License:Expat>.

# Copyright 2023 freiheit.com---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: kuberpult-cd-service
  labels:
    app: kuberpult-cd-service
spec:
  # Generally, it is possible to have multiple instances of the cd-service.
  # However, most time is spent in a ` + "`git push`" + `, which cannot be parallelized much.
  # Having multiple instances works when there are only few requests/sec,
  # but it may get inefficient if there are many, since kuberpult then needs
  # to ` + "`pull`" + ` and ` + "`push`" + ` more often due to possible conflicts.
  # Therefore, we only allow 1 instance of the cd-service.
  # If you temporarily need 2, that will also work.
  replicas: 1
  selector:
    matchLabels:
      app: kuberpult-cd-service
  template:
    metadata:
      labels:
        app: kuberpult-cd-service
    spec:
      containers:
      - name: service
        image: "europe-west3-docker.pkg.dev/fdc-public-docker-registry/kuberpult/kuberpult-cd-service:v2.21.0-2-ga83c7de"
        ports:
          - name: http
            containerPort: 8080
            protocol: TCP
          - name: grpc
            containerPort: 8443
            protocol: TCP
        readinessProbe:
          httpGet:
            path: /healthz
            port: http
          initialDelaySeconds: 5
          periodSeconds: 10
          successThreshold: 1
          timeoutSeconds: 5
          failureThreshold: 10
        livenessProbe:
          httpGet:
            path: /healthz
            port: http
          initialDelaySeconds: 5
          periodSeconds: 10
          successThreshold: 1
          timeoutSeconds: 5
          failureThreshold: 10
        resources:
          limits:
            cpu: "2"
            memory: "3Gi"
          requests:
            cpu: "2"
            memory: "3Gi"
        env:
        - name: KUBERPULT_GIT_URL
          value: "git@github.com/.../..."
        - name: KUBERPULT_GIT_BRANCH
          value: "master"
        - name: LOG_FORMAT
          value: ""
        - name: LOG_LEVEL
          value: "WARN"
        - name: KUBERPULT_ARGO_CD_SERVER
          value: ""
        - name: KUBERPULT_ARGO_CD_INSECURE
          value: "false"
        - name: KUBERPULT_ARGO_CD_GENERATE_FILES
          value: "true"
        - name: KUBERPULT_GIT_WEB_URL
          value: 
        - name: KUBERPULT_AZURE_ENABLE_AUTH
          value: "false"
        - name: KUBERPULT_DEX_ENABLED
          value: "false"
        - name: KUBERPULT_ENABLE_SQLITE
          value: "true"
        - name: KUBERPULT_GIT_NETWORK_TIMEOUT
          value: "1m"
        - name: KUBERPULT_GIT_WRITE_COMMIT_DATA
          value: "false"
        - name: KUBERPULT_GIT_MAXIMUM_COMMITS_PER_PUSH
          value: "5"
        - name: KUBERPULT_ENABLE_PROFILING
          value: "false"
        - name: KUBERPULT_MAXIMUM_QUEUE_SIZE
          value: "5"
        volumeMounts:
        - name: repository
          mountPath: /repository
        - name: ssh
          mountPath: /etc/ssh
      volumes:
      - name: repository
        # We use emptyDir, because none of our data needs to survive for long (it's all in the github repo).
        # EmptyDir has the nice advantage, that it triggers a restart of the pod and creates a new volume when the current one is full
        # Because of an issue in gitlib2, this actually happens.
        emptyDir:
          sizeLimit: 10Gi
      - name: ssh
        secret:
          secretName: kuberpult-ssh
---
# Source: kuberpult/templates/frontend-service.yaml
# This file is part of kuberpult.

# Kuberpult is free software: you can redistribute it and/or modify
# it under the terms of the Expat(MIT) License as published by
# the Free Software Foundation.

# Kuberpult is distributed in the hope that it will be useful,
# but WITHOUT ANY WARRANTY; without even the implied warranty of
# MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
# MIT License for more details.

# You should have received a copy of the MIT License
# along with kuberpult. If not, see <https://directory.fsf.org/wiki/License:Expat>.

# Copyright 2023 freiheit.com
# This file is part of kuberpult.

# Kuberpult is free software: you can redistribute it and/or modify
# it under the terms of the Expat(MIT) License as published by
# the Free Software Foundation.

# Kuberpult is distributed in the hope that it will be useful,
# but WITHOUT ANY WARRANTY; without even the implied warranty of
# MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
# MIT License for more details.

# You should have received a copy of the MIT License
# along with kuberpult. If not, see <https://directory.fsf.org/wiki/License:Expat>.

# Copyright 2023 freiheit.com---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: kuberpult-frontend-service
  labels:
    app: kuberpult-frontend-service
spec:
  replicas: 2
  selector:
    matchLabels:
      app: kuberpult-frontend-service
  template:
    metadata:
      labels:
        app: kuberpult-frontend-service
    spec:
      containers:
      - name: service
        image: "europe-west3-docker.pkg.dev/fdc-public-docker-registry/kuberpult/kuberpult-frontend-service:v2.21.0-2-ga83c7de"
        ports:
          - name: http
            containerPort: 8081
            protocol: TCP
        readinessProbe:
          httpGet:
            path: /healthz
            port: http
          initialDelaySeconds: 5
          periodSeconds: 10
        livenessProbe:
          httpGet:
            path: /healthz
            port: http
        resources:
          limits:
            cpu: "500m"
            memory: "250Mi"
          requests:
            cpu: "500m"
            memory: "250Mi"
        env:
        - name: KUBERPULT_GIT_AUTHOR_NAME
          value: "local.user@example.com"
        - name: KUBERPULT_GIT_AUTHOR_EMAIL
          value: "defaultUser"
        - name: KUBERPULT_CDSERVER
          value: kuberpult-cd-service:8443
        - name: KUBERPULT_ARGOCD_BASE_URL
          value: ""
        - name: KUBERPULT_ARGOCD_NAMESPACE
          value: 
        - name: KUBERPULT_BATCH_CLIENT_TIMEOUT
          value: "2m"
        - name: KUBERPULT_VERSION
          value: "v2.21.0-2-ga83c7de"
        - name: KUBERPULT_SOURCE_REPO_URL
          value: ""
        - name: KUBERPULT_MANIFEST_REPO_URL
          value: ""
        - name: LOG_FORMAT
          value: ""
        - name: LOG_LEVEL
          value: "WARN"
        - name: KUBERPULT_GKE_BACKEND_SERVICE_ID
          value: ""
        - name: KUBERPULT_GKE_BACKEND_SERVICE_NAME
          value: ""
        - name: KUBERPULT_GKE_PROJECT_NUMBER
          value: ""
        - name: KUBERPULT_ALLOWED_ORIGINS
          value: "https://"
        - name: KUBERPULT_GIT_BRANCH
          value: "master"
        - name: KUBERPULT_IAP_ENABLED
          value: "false"
        - name: KUBERPULT_API_ENABLE_DESPITE_NO_AUTH
          value: "false"
        - name: KUBERPULT_DEX_ENABLED
          value: "false"
        - name: KUBERPULT_AZURE_ENABLE_AUTH
          value: "false"
        - name: KUBERPULT_ROLLOUTSERVER
          value: ""
        - name: KUBERPULT_MAX_WAIT_DURATION
          value: "10m"
        volumeMounts:
      volumes:
---
# Source: kuberpult/templates/ingress.yaml
# This file is part of kuberpult.

# Kuberpult is free software: you can redistribute it and/or modify
# it under the terms of the Expat(MIT) License as published by
# the Free Software Foundation.

# Kuberpult is distributed in the hope that it will be useful,
# but WITHOUT ANY WARRANTY; without even the implied warranty of
# MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
# MIT License for more details.

# You should have received a copy of the MIT License
# along with kuberpult. If not, see <https://directory.fsf.org/wiki/License:Expat>.

# Copyright 2023 freiheit.com
# This file is part of kuberpult.

# Kuberpult is free software: you can redistribute it and/or modify
# it under the terms of the Expat(MIT) License as published by
# the Free Software Foundation.

# Kuberpult is distributed in the hope that it will be useful,
# but WITHOUT ANY WARRANTY; without even the implied warranty of
# MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
# MIT License for more details.

# You should have received a copy of the MIT License
# along with kuberpult. If not, see <https://directory.fsf.org/wiki/License:Expat>.

# Copyright 2023 freiheit.com
# This file is part of kuberpult.

# Kuberpult is free software: you can redistribute it and/or modify
#it under the terms of the GNU General Public License as published by
#the Free Software Foundation, either version 3 of the License, or
#(at your option) any later version.

# Kuberpult is distributed in the hope that it will be useful,
# but WITHOUT ANY WARRANTY; without even the implied warranty of
# MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
#GNU General Public License for more details.

#You should have received a copy of the GNU General Public License
#along with kuberpult.  If not, see <http://www.gnu.org/licenses/>.

#Copyright 2022 freiheit.com
---
# Source: kuberpult/templates/rollout-service.yaml
# This file is part of kuberpult.

# Kuberpult is free software: you can redistribute it and/or modify
# it under the terms of the Expat(MIT) License as published by
# the Free Software Foundation.

# Kuberpult is distributed in the hope that it will be useful,
# but WITHOUT ANY WARRANTY; without even the implied warranty of
# MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
# MIT License for more details.

# You should have received a copy of the MIT License
# along with kuberpult. If not, see <https://directory.fsf.org/wiki/License:Expat>.

# Copyright 2023 freiheit.com
# This file is part of kuberpult.
`

var dd_enabled_expected_output = `---
# Source: kuberpult/templates/cd-service.yaml
apiVersion: v1
kind: Secret
metadata:
  name: kuberpult-ssh
data:
  identity: "LS0tLS1CRUdJTiBPUEVOU1NIIFBSSVZBVEUgS0VZLS0tLS0KLS0tLS1FTkQgT1BFTlNTSCBQUklWQVRFIEtFWS0tLS0tCg=="
  ssh_known_hosts: "Z2l0aHViLmNvbSBlY2RzYS1zaGEyLW5pc3RwMjU2IEFBQUFFMlZqWkhOaExYTm9ZVEl0Ym1semRIQXlOVFlBQUFBSWJtbHpkSEF5TlRZQUFBQkJCRW1LU0VOalFFZXpPbXhrWk15N29wS2d3RkI5bmt0NVlScllNak51RzVOODd1UmdnNkNMcmJvNXdBZFQveTZ2MG1LVjBVMncwV1oyWUIvKytUcG9ja2c9Cg=="
---
# Source: kuberpult/templates/cd-service.yaml
apiVersion: v1
kind: Service
metadata:
  name: kuberpult-cd-service
spec:
  ports:
  - name: http
    port: 80
    targetPort: http
  - name: grpc
    port: 8443
    targetPort: grpc
  selector:
    app: kuberpult-cd-service
  type: NodePort
---
# Source: kuberpult/templates/frontend-service.yaml
apiVersion: v1
kind: Service
metadata:
  name: kuberpult-frontend-service
  annotations:
spec:
  ports:
  - name: http
    port: 80
    targetPort: http
  selector:
    app: kuberpult-frontend-service
  type: NodePort
---
# Source: kuberpult/templates/cd-service.yaml
# This file is part of kuberpult.

# Kuberpult is free software: you can redistribute it and/or modify
# it under the terms of the Expat(MIT) License as published by
# the Free Software Foundation.

# Kuberpult is distributed in the hope that it will be useful,
# but WITHOUT ANY WARRANTY; without even the implied warranty of
# MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
# MIT License for more details.

# You should have received a copy of the MIT License
# along with kuberpult. If not, see <https://directory.fsf.org/wiki/License:Expat>.

# Copyright 2023 freiheit.com
# This file is part of kuberpult.

# Kuberpult is free software: you can redistribute it and/or modify
# it under the terms of the Expat(MIT) License as published by
# the Free Software Foundation.

# Kuberpult is distributed in the hope that it will be useful,
# but WITHOUT ANY WARRANTY; without even the implied warranty of
# MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
# MIT License for more details.

# You should have received a copy of the MIT License
# along with kuberpult. If not, see <https://directory.fsf.org/wiki/License:Expat>.

# Copyright 2023 freiheit.com---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: kuberpult-cd-service
  labels:
    app: kuberpult-cd-service
spec:
  # Generally, it is possible to have multiple instances of the cd-service.
  # However, most time is spent in a ` + "`git push`" + `, which cannot be parallelized much.
  # Having multiple instances works when there are only few requests/sec,
  # but it may get inefficient if there are many, since kuberpult then needs
  # to ` + "`pull`" + ` and ` + "`push`" + ` more often due to possible conflicts.
  # Therefore, we only allow 1 instance of the cd-service.
  # If you temporarily need 2, that will also work.
  replicas: 1
  selector:
    matchLabels:
      app: kuberpult-cd-service
  template:
    metadata:
      labels:
        app: kuberpult-cd-service
    spec:
      containers:
      - name: service
        image: "europe-west3-docker.pkg.dev/fdc-public-docker-registry/kuberpult/kuberpult-cd-service:v2.21.0-2-ga83c7de"
        ports:
          - name: http
            containerPort: 8080
            protocol: TCP
          - name: grpc
            containerPort: 8443
            protocol: TCP
        readinessProbe:
          httpGet:
            path: /healthz
            port: http
          initialDelaySeconds: 5
          periodSeconds: 10
          successThreshold: 1
          timeoutSeconds: 5
          failureThreshold: 10
        livenessProbe:
          httpGet:
            path: /healthz
            port: http
          initialDelaySeconds: 5
          periodSeconds: 10
          successThreshold: 1
          timeoutSeconds: 5
          failureThreshold: 10
        resources:
          limits:
            cpu: "2"
            memory: "3Gi"
          requests:
            cpu: "2"
            memory: "3Gi"
        env:
        - name: KUBERPULT_GIT_URL
          value: "git@github.com/.../..."
        - name: KUBERPULT_GIT_BRANCH
          value: "master"
        - name: LOG_FORMAT
          value: ""
        - name: LOG_LEVEL
          value: "WARN"
        - name: KUBERPULT_ARGO_CD_SERVER
          value: ""
        - name: KUBERPULT_ARGO_CD_INSECURE
          value: "false"
        - name: KUBERPULT_ARGO_CD_GENERATE_FILES
          value: "true"
        - name: KUBERPULT_GIT_WEB_URL
          value: 
        - name: KUBERPULT_ENABLE_METRICS
          value: "true"
        - name: KUBERPULT_ENABLE_EVENTS
          value: "false"
        - name: KUBERPULT_DOGSTATSD_ADDR
          value: "unix:///var/run/datadog/dsd.socket"
        - name: KUBERPULT_AZURE_ENABLE_AUTH
          value: "false"
        - name: KUBERPULT_DEX_ENABLED
          value: "false"
        - name: KUBERPULT_ENABLE_SQLITE
          value: "true"
        - name: KUBERPULT_GIT_NETWORK_TIMEOUT
          value: "1m"
        - name: KUBERPULT_GIT_WRITE_COMMIT_DATA
          value: "false"
        - name: KUBERPULT_GIT_MAXIMUM_COMMITS_PER_PUSH
          value: "5"
        - name: KUBERPULT_ENABLE_PROFILING
          value: "false"
        - name: KUBERPULT_MAXIMUM_QUEUE_SIZE
          value: "5"
        volumeMounts:
        - name: repository
          mountPath: /repository
        - name: ssh
          mountPath: /etc/ssh
        - name: dsdsocket
          mountPath: /var/run/datadog
          readOnly: true
      volumes:
      - name: repository
        # We use emptyDir, because none of our data needs to survive for long (it's all in the github repo).
        # EmptyDir has the nice advantage, that it triggers a restart of the pod and creates a new volume when the current one is full
        # Because of an issue in gitlib2, this actually happens.
        emptyDir:
          sizeLimit: 10Gi
      - name: ssh
        secret:
          secretName: kuberpult-ssh
      - name: dsdsocket
        hostPath:
          path: /var/run/datadog
---
# Source: kuberpult/templates/frontend-service.yaml
# This file is part of kuberpult.

# Kuberpult is free software: you can redistribute it and/or modify
# it under the terms of the Expat(MIT) License as published by
# the Free Software Foundation.

# Kuberpult is distributed in the hope that it will be useful,
# but WITHOUT ANY WARRANTY; without even the implied warranty of
# MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
# MIT License for more details.

# You should have received a copy of the MIT License
# along with kuberpult. If not, see <https://directory.fsf.org/wiki/License:Expat>.

# Copyright 2023 freiheit.com
# This file is part of kuberpult.

# Kuberpult is free software: you can redistribute it and/or modify
# it under the terms of the Expat(MIT) License as published by
# the Free Software Foundation.

# Kuberpult is distributed in the hope that it will be useful,
# but WITHOUT ANY WARRANTY; without even the implied warranty of
# MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
# MIT License for more details.

# You should have received a copy of the MIT License
# along with kuberpult. If not, see <https://directory.fsf.org/wiki/License:Expat>.

# Copyright 2023 freiheit.com---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: kuberpult-frontend-service
  labels:
    app: kuberpult-frontend-service
spec:
  replicas: 2
  selector:
    matchLabels:
      app: kuberpult-frontend-service
  template:
    metadata:
      labels:
        app: kuberpult-frontend-service
    spec:
      containers:
      - name: service
        image: "europe-west3-docker.pkg.dev/fdc-public-docker-registry/kuberpult/kuberpult-frontend-service:v2.21.0-2-ga83c7de"
        ports:
          - name: http
            containerPort: 8081
            protocol: TCP
        readinessProbe:
          httpGet:
            path: /healthz
            port: http
          initialDelaySeconds: 5
          periodSeconds: 10
        livenessProbe:
          httpGet:
            path: /healthz
            port: http
        resources:
          limits:
            cpu: "500m"
            memory: "250Mi"
          requests:
            cpu: "500m"
            memory: "250Mi"
        env:
        - name: KUBERPULT_GIT_AUTHOR_NAME
          value: "local.user@example.com"
        - name: KUBERPULT_GIT_AUTHOR_EMAIL
          value: "defaultUser"
        - name: KUBERPULT_CDSERVER
          value: kuberpult-cd-service:8443
        - name: KUBERPULT_ARGOCD_BASE_URL
          value: ""
        - name: KUBERPULT_ARGOCD_NAMESPACE
          value: 
        - name: KUBERPULT_BATCH_CLIENT_TIMEOUT
          value: "2m"
        - name: KUBERPULT_VERSION
          value: "v2.21.0-2-ga83c7de"
        - name: KUBERPULT_SOURCE_REPO_URL
          value: ""
        - name: KUBERPULT_MANIFEST_REPO_URL
          value: ""
        - name: LOG_FORMAT
          value: ""
        - name: LOG_LEVEL
          value: "WARN"
        - name: KUBERPULT_GKE_BACKEND_SERVICE_ID
          value: ""
        - name: KUBERPULT_GKE_BACKEND_SERVICE_NAME
          value: ""
        - name: KUBERPULT_GKE_PROJECT_NUMBER
          value: ""
        - name: KUBERPULT_ALLOWED_ORIGINS
          value: "https://"
        - name: KUBERPULT_GIT_BRANCH
          value: "master"
        - name: KUBERPULT_IAP_ENABLED
          value: "false"
        - name: KUBERPULT_API_ENABLE_DESPITE_NO_AUTH
          value: "false"
        - name: KUBERPULT_DEX_ENABLED
          value: "false"
        - name: KUBERPULT_AZURE_ENABLE_AUTH
          value: "false"
        - name: KUBERPULT_ROLLOUTSERVER
          value: ""
        - name: KUBERPULT_MAX_WAIT_DURATION
          value: "10m"
        volumeMounts:
      volumes:
---
# Source: kuberpult/templates/ingress.yaml
# This file is part of kuberpult.

# Kuberpult is free software: you can redistribute it and/or modify
# it under the terms of the Expat(MIT) License as published by
# the Free Software Foundation.

# Kuberpult is distributed in the hope that it will be useful,
# but WITHOUT ANY WARRANTY; without even the implied warranty of
# MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
# MIT License for more details.

# You should have received a copy of the MIT License
# along with kuberpult. If not, see <https://directory.fsf.org/wiki/License:Expat>.

# Copyright 2023 freiheit.com
# This file is part of kuberpult.

# Kuberpult is free software: you can redistribute it and/or modify
# it under the terms of the Expat(MIT) License as published by
# the Free Software Foundation.

# Kuberpult is distributed in the hope that it will be useful,
# but WITHOUT ANY WARRANTY; without even the implied warranty of
# MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
# MIT License for more details.

# You should have received a copy of the MIT License
# along with kuberpult. If not, see <https://directory.fsf.org/wiki/License:Expat>.

# Copyright 2023 freiheit.com
# This file is part of kuberpult.

# Kuberpult is free software: you can redistribute it and/or modify
#it under the terms of the GNU General Public License as published by
#the Free Software Foundation, either version 3 of the License, or
#(at your option) any later version.

# Kuberpult is distributed in the hope that it will be useful,
# but WITHOUT ANY WARRANTY; without even the implied warranty of
# MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
#GNU General Public License for more details.

#You should have received a copy of the GNU General Public License
#along with kuberpult.  If not, see <http://www.gnu.org/licenses/>.

#Copyright 2022 freiheit.com
---
# Source: kuberpult/templates/rollout-service.yaml
# This file is part of kuberpult.

# Kuberpult is free software: you can redistribute it and/or modify
# it under the terms of the Expat(MIT) License as published by
# the Free Software Foundation.

# Kuberpult is distributed in the hope that it will be useful,
# but WITHOUT ANY WARRANTY; without even the implied warranty of
# MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
# MIT License for more details.

# You should have received a copy of the MIT License
# along with kuberpult. If not, see <https://directory.fsf.org/wiki/License:Expat>.

# Copyright 2023 freiheit.com
# This file is part of kuberpult.
`
var dd_enabled_values = `# This file is part of kuberpult.

# Kuberpult is free software: you can redistribute it and/or modify
# it under the terms of the Expat(MIT) License as published by
# the Free Software Foundation.

# Kuberpult is distributed in the hope that it will be useful,
# but WITHOUT ANY WARRANTY; without even the implied warranty of
# MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
# MIT License for more details.

# You should have received a copy of the MIT License
# along with kuberpult. If not, see <https://directory.fsf.org/wiki/License:Expat>.

# Copyright 2023 freiheit.com
# Default values for ..
# This is a YAML-formatted file.
# Declare variables to be passed into your templates.

git:
  # The git url of the manifest repository (git protocol)
  url: git@github.com/.../...
  webUrl:  # only necessary for webhooks to argoCd, e.g. https://github.com/freiheit-com/kuberpult

  # The branch to be use in the manifest repository
  branch: "master"

  manifestRepoUrl: ""


  sourceRepoUrl: ""

  # The git author is what kuberpult writes to the manifest repository.
  # The git committer cannot be configured. It will always be "kuberpult".
  author:
    name: local.user@example.com
    email: defaultUser

  # Timeout used for network operations
  networkTimeout: 1m


  enableWritingCommitData: false

  # Kuberpult tries to reduce the number of pushes and can bundle concurrent commits into a single push.
  # This can reduce the time it takes to process requests ariving at the same time and improve throughput.
  # The correct number largely depends on the performance of the git host and repository size. For small to medium sized deployments the default is good.
  # We recommend values between 1 and 20.
  maximumCommitsPerPush: 5

hub: europe-west3-docker.pkg.dev/fdc-public-docker-registry/kuberpult

log:
  # Possible values are "gcp" for a gcp-optimized format and "default" for json
  format: ""
  # Other possible values are "DEBUG", "INFO", "ERROR"
  level: "WARN"
cd:
  image: kuberpult-cd-service
  backendConfig:
    create: false   # Add backend config for health checks on GKE only
    timeoutSec: 300  # 30sec is the default on gcp loadbalancers, however kuberpult needs more with parallel requests. It is the time how long the loadbalancer waits for kuberpult to finish calls to the rest endpoint "release"
    queueSize: 5
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
      periodSeconds: 10
      successThreshold: 1
      timeoutSeconds: 5
      failureThreshold: 10
      initialDelaySeconds: 5
    readiness:
      periodSeconds: 10
      successThreshold: 1
      timeoutSeconds: 5
      failureThreshold: 10
      initialDelaySeconds: 5
frontend:
  image: kuberpult-frontend-service
# Annotations given here will be added to kuberpult-frontend-service annotations.
# See frontend-service.yaml for automatically added annotations.
  service:
    annotations: {}
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
  batchClient:
    # This value needs to be higher than the network timeout for git
    timeout: 2m
rollout:
  enabled: false
  image: kuberpult-rollout-service
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
  create: false
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

  generateFiles: true

datadogTracing:
  enabled: false
  debugging: false
  environment: "shared"

datadogProfiling:
  apiKey: ""

dogstatsdMetrics:
  # send metrics:
  enabled: true

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
  api:
    enableDespiteNoAuth: false

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
`

func TestServeHttpBasics(t *testing.T) {
	tcs := []struct {
		Name           string
		values         string
		expectedOutput string
	}{
		{
			Name:           "Test1",
			values:         tpl_dd_disabled_values,
			expectedOutput: tpl_dd_disabled_expected_output,
		},
		{
			Name:           "Test2",
			values:         dd_enabled_values,
			expectedOutput: dd_enabled_expected_output,
		},
	}
	dirName := "testfiles"
	defer os.RemoveAll("testfiles")
	if err := os.Mkdir(dirName, os.ModePerm); err != nil {
		t.Fatalf("Could not create test file dir! %v", err)
	}
	for _, tc := range tcs {
		tc := tc
		t.Run(tc.Name, func(t *testing.T) {
			testId := strconv.Itoa(rand.IntN(100))
			valsFile := "vals" + "_" + testId + ".yaml"
			outputFile := "tmp_" + testId + ".tmpl"
			outputFile = dirName + "/" + outputFile
			valsFile = dirName + "/" + valsFile
			d1 := []byte(tc.values)
			err := os.WriteFile(valsFile, d1, 0644)
			if err != nil {
				t.Fatalf("Error writing vals file . %v", err)
			}
			output, err := exec.Command("sh", "-c", "helm template ./ --values "+valsFile+" > "+outputFile).CombinedOutput()

			//Define the values .yaml
			//Run helm
			//Check that the output contains what we want, for example if values.yaml has x set to true, we shoud expect that the tmpl file has some field to "example", if not, we
			if err != nil {
				fmt.Println("Error when running command.  Output:")
				fmt.Println(string(output))
				fmt.Printf("Got command status: %s\n", err.Error())
				t.Fatal()
				return

			}

			actualOutput, err := os.ReadFile(outputFile)
			if err != nil {
				t.Fatal("Error reading output file.")
			}
			d := cmp.Diff(tc.expectedOutput, string(actualOutput))
			if d != "" {
				t.Errorf("unexpected diff between config maps: %s", d)
			}
			fmt.Println("Success")
		})
	}
}
