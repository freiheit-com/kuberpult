#!/bin/bash
set -e
set -o pipefail

# usage
# ./create-releases.sh base-app-name team number-releases
# Note that this just creates files, it doesn't push in git

BASE_APP_NAME=${1}
applicationOwnerTeam=${2:-sreteam}
NUMBER_RELEASES=${3:-100}

# 40 is the length of a full git commit hash.
authors[0]="urbansky"
authors[1]="Medo"
authors[2]="Hannes"
authors[3]="Mouhsen"
authors[4]="Tamer"
authors[5]="Ahmed"
authors[6]="João"
authors[7]="Leandro"
sizeAuthors=${#authors[@]}
index=$(($RANDOM % $sizeAuthors))
echo ${authors[$index]}
author="${authors[$index]}"
commit_message_file="$(mktemp "${TMPDIR:-/tmp}/publish.XXXXXX")"
trap "rm -f ""$commit_message_file" INT TERM HUP EXIT


msgs[0]="Added new eslint rule"
msgs[1]="Fix annotations in helm templates"
msgs[2]="Improve performance with gitLib2"
msgs[3]="Add colors to new UI"
msgs[4]="Release trains for env groups"
msgs[5]="Fix whitespace in ReleaseDialog"
msgs[6]="Add Snackbar Notifications"
msgs[7]="Rephrase ReleaseDialog text"
msgs[8]="Change renovate schedule"
msgs[9]="Fix bug in distanceToUpstream calculation"
msgs[10]="Allow deleting locks on locks page"
sizeMsgs=${#msgs[@]}
index=$(($RANDOM % $sizeMsgs))
echo $index
echo -e "${msgs[$index]}\n" > "${commit_message_file}"
echo "1: ${msgs[$index]}" >> "${commit_message_file}"
echo "2: ${msgs[$index]}" >> "${commit_message_file}"

ls "${commit_message_file}"

release_version=''
case "${RELEASE_VERSION:-}" in
	*[!0-9]*) echo "Please set the env variable RELEASE_VERSION to a number"; exit 1;;
	*) release_version='--form-string '"version=${RELEASE_VERSION:-}";;
esac


qaEnvs=()
configuration+=("--form" "team=${applicationOwnerTeam}")
search_dir=environments
for env in "$search_dir"/*
do
  env=$(basename "${env}")
  env=$(echo "$env" | awk '{print tolower($0)}')
  if [ "$env" != "testing" ]; then
    qaEnvs+=( "$env" )
  fi
done

FRONTEND_PORT=8081 # see docker-compose.yml

for (( c=0; c<=NUMBER_RELEASES; c++ ))
do
  deployments=$(($RANDOM % $sizeAuthors))
  n_deployments=$((10 + $deployments +1))
  run_release_train=$(($RANDOM % $n_deployments))

  for (( d=1; d<=n_deployments; d++ ))
  do
    commit_id=$(head -c 20 /dev/urandom | sha1sum | awk '{print $1}') # SHA-1 produces 40-character hashes
    inputs=()
    #inputs+=(--form-string "application=$name")
    inputs+=(--form-string "source_commit_id=$commit_id")
    inputs+=(--form-string "source_author=$author")
    manifests=()
    search_dir=environments

    for env in "$search_dir"/*
    do
      env=$(basename "${env}")
      env=$(echo "$env" | awk '{print tolower($0)}')
      signatureFile=$(mktemp "${TMPDIR:-/tmp}/$env.XXXXXX")
      file=$(mktemp "${x:-/tmp}/$env.XXXXXX")
      randomValue=$(head -c 20 /dev/urandom | sha1sum | awk '{print $1}' | head -c 12)
    cat <<EOF > "${file}"
---
apiVersion: v1
data:
  TNS_ADMIN: /db-config
  TESTDB_SID_LOOKUP_FILENAME: /db-config/locations.json
kind: ConfigMap
metadata:
  name: foo-service
  namespace: md
---
apiVersion: v1
kind: Service
metadata:
  name: foo-service
  namespace: md
spec:
  ports:
  - name: grpc
    port: 50051
    targetPort: 50051
  - name: health
    port: 8080
    targetPort: 8080
  selector:
    app: foo-service
  type: ClusterIP
---
apiVersion: apps/v1
kind: Deployment
metadata:
  labels:
    app: foo-service
    service: md.foo-service
    tags.datadoghq.com/env: qas
    tags.datadoghq.com/service: md.foo-service
    tags.datadoghq.com/version: c544a05b09237b49b5cec53e4de6d9ae85e794c9
    version: c544a05b09237b49b5cec53e4de6d9ae85e794c9
  name: foo-service
  namespace: md
spec:
  replicas: 1
  revisionHistoryLimit: 3
  selector:
    matchLabels:
      app: foo-service
  strategy:
    rollingUpdate:
      maxSurge: 1
      maxUnavailable: 0
    type: RollingUpdate
  template:
    metadata:
      annotations:
        ad.datadoghq.com/tags: '{"team": "md-supplier"}'
        apm.datadoghq.com/env: '{"DD_SERVICE":"md.foo-service","DD_VERSION":"c544a05b09237b49b5cec53e4de6d9ae85e794c9","DD_PROPAGATION_STYLE_INJECT":"B3","DD_PROPAGATION_STYLE_EXTRACT":"B3"}'
        sidecar.istio.io/agentLogLevel: default:error,xdsproxy:error,ads:error,cache:error,sds:error
        sidecar.istio.io/componentLogLevel: misc:error
        sidecar.istio.io/logLevel: error
      labels:
        app: foo-service
        service: md.foo-service
        service.istio.io/canonical-name: md.foo-service
        sidecar.istio.io/inject: "true"
        tags.datadoghq.com/env: qas
        tags.datadoghq.com/service: md.foo-service
        tags.datadoghq.com/version: c544a05b09237b49b5cec53e4de6d9ae85e794c9
        version: c544a05b09237b49b5cec53e4de6d9ae85e794c9
    spec:
      containers:
      - env:
        - name: POD_IP
          valueFrom:
            fieldRef:
              fieldPath: status.podIP
        - name: POD_NAME
          valueFrom:
            fieldRef:
              fieldPath: metadata.name
        - name: SERVICE_NAME
          value: foo-service
        - name: DOMAIN
          value: md
        - name: TEAM
          value: md-supplier
        - name: ENV
          value: qa
        - name: HOST_IP
          valueFrom:
            fieldRef:
              fieldPath: status.hostIP
        - name: DD_ENV
          valueFrom:
            fieldRef:
              fieldPath: metadata.labels['tags.datadoghq.com/env']
        - name: DD_SERVICE
          valueFrom:
            fieldRef:
              fieldPath: metadata.labels['tags.datadoghq.com/service']
        - name: DD_VERSION
          valueFrom:
            fieldRef:
              fieldPath: metadata.labels['tags.datadoghq.com/version']
        - name: DD_CLUSTER_NAME
          valueFrom:
            configMapKeyRef:
              key: CLUSTER_NAME
              name: shared-config
              optional: true
        - name: ISTIO_ENDPOINT
          value: http://127.0.0.1:15000
        - name: DD_TRACE_AGENT_URL
          value: unix:///var/run/datadog/apm.socket
        - name: DD_DOGSTATSD_SOCKET
          value: /var/run/datadog/dsd.socket
        - name: DD_TRACE_PROPAGATION_STYLE
          value: b3multi
        - name: DD_TRACE_SAMPLE_RATE
          value: "1.0"
        envFrom:
        - configMapRef:
            name: shared-config
            optional: true
        - secretRef:
            name: shared-config
            optional: true
        - secretRef:
            name: foo-service-terraform
            optional: true
        - configMapRef:
            name: foo-service
            optional: true
        - secretRef:
            name: foo-service
            optional: true
        image: example.com/domains/md/services/foo-service:foobar
        imagePullPolicy: IfNotPresent
        livenessProbe:
          httpGet:
            path: /live
            port: 8081
          initialDelaySeconds: 5
          periodSeconds: 5
        name: foo-service
        ports:
        - containerPort: 8080
          name: http
        - containerPort: 8081
          name: health
        - containerPort: 50051
          name: grpc
        readinessProbe:
          failureThreshold: 2
          httpGet:
            path: /ready
            port: 8081
          initialDelaySeconds: 5
          periodSeconds: 5
          successThreshold: 1
        resources:
          limits:
            cpu: 150m
            memory: 100Mi
          requests:
            cpu: 50m
            memory: 100Mi
        startupProbe:
          failureThreshold: 60
          httpGet:
            path: /startup
            port: 8081
          initialDelaySeconds: 15
          periodSeconds: 5
        volumeMounts:
        - mountPath: /db-config
          name: oracle-location-config
        - mountPath: /etc/testdb
          name: oracle-db-users
          readOnly: true
        - mountPath: /var/run/datadog
          name: dsdsocket
          readOnly: true
        - mountPath: /etc/service-user
          name: service-user-secret
          readOnly: true
      securityContext:
        runAsNonRoot: true
      topologySpreadConstraints:
      - labelSelector:
          matchLabels:
            app: foo-service
        maxSkew: 1
        topologyKey: kubernetes.io/hostname
        whenUnsatisfiable: ScheduleAnyway
      volumes:
      - configMap:
          name: oracle-location-config
        name: oracle-location-config
      - name: oracle-db-users
        secret:
          secretName: oracle-db-users
      - hostPath:
          path: /var/run/datadog/
        name: dsdsocket
      - name: service-user-secret
        secret:
          items:
          - key: md-foo-service-service-user-secret
            path: secret-a
          - key: md-foo-service-service-user-secret-old
            path: secret-b
          optional: true
          secretName: md-service-user-secrets
---
apiVersion: policy/v1
kind: PodDisruptionBudget
metadata:
  name: foo-service-pdb
  namespace: md
spec:
  maxUnavailable: 25%
  selector:
    matchLabels:
      app: foo-service
---
apiVersion: autoscaling/v2
kind: HorizontalPodAutoscaler
metadata:
  labels:
    app: foo-service
  name: foo-service
  namespace: md
spec:
  maxReplicas: 6
  metrics:
  - resource:
      name: cpu
      target:
        averageUtilization: 70
        type: Utilization
    type: Resource
  minReplicas: 1
  scaleTargetRef:
    apiVersion: apps/v1
    kind: Deployment
    name: foo-service
---
apiVersion: networking.istio.io/v1beta1
kind: VirtualService
metadata:
  name: foo-service
  namespace: md
spec:
  hosts:
  - foo-service
  http:
  - route:
    - destination:
        host: foo-service
        port:
          number: 50051
  random: "${randomValue}"
---
EOF
      manifests+=("--form" "manifests[${env}]=@${file}")
      gpg  --keyring trustedkeys-kuberpult.gpg --local-user kuberpult-kind@example.com --detach --sign --armor < "${file}" > "${signatureFile}"
      manifests+=("--form" "signatures[${env}]=@${signatureFile}")
    done
    if [[ $(uname -o) == Darwin ]];
    then
      EMAIL=$(echo -n "script-user@example.com" | base64 -b 0)
      AUTHOR=$(echo -n "script-user" | base64 -b 0)
    else
      EMAIL=$(echo -n "script-user@example.com" | base64 -w 0)
      AUTHOR=$(echo -n "script-user" | base64 -w 0)
    fi
    commit_message=$(head -c 8192 /dev/urandom | base64 | awk '{print $1}' | head -c 128)
    displayVersion=
    if ((RANDOM % 2)); then
      displayVersion=$(( $RANDOM % 100)).$(( $RANDOM % 100)).$(( $RANDOM % 100))
    fi
    index=$(($RANDOM % $sizeAuthors))
    author="${authors[$index]}"
      time curl http://localhost:${FRONTEND_PORT}/release \
        -H "author-email:${EMAIL}" \
        -H "author-name:${AUTHOR}=" \
        "${inputs[@]}" \
        --form-string "version=$d" \
        --form-string "display_version=${displayVersion}" \
        --form-string "application=$BASE_APP_NAME-$c" \
        --form "source_message=${commit_message}" \
        "${configuration[@]}" \
        "${manifests[@]}"
        echo Created Version "$d" of "$BASE_APP_NAME-$c"
    if [ "$d" -eq $run_release_train ];
    then
      ./../run-releasetrain.sh qa
    fi
  done
  for t in ${qaEnvs[@]}; do
    ./../create-app-lock.sh "$BASE_APP_NAME-$c" "$t"
  done
done