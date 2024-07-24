#!/usr/bin/env bash

set -eu
set -o pipefail

function print() {
  /bin/echo "$0:" "$@"
}

function waitForDeployment() {
  ns="$1"
  label="$2"
  echo "waitForDeployment: $ns/$label"
  until kubectl wait --for=condition=ready pod -n "$ns" -l "$label" --timeout=30s;
  do
    sleep 4
    echo "logs:"
    kubectl -n "$ns" logs -l "$label" || echo "could not get logs for $label"
    echo "describe pod:"
    kubectl -n "$ns" describe pod -l "$label"
    echo ...
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

kubectl apply -f - <<EOF
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: postgres-config
  labels:
    app: postgres
data:
  POSTGRES_DB: kuberpult
  POSTGRES_PASSWORD: mypassword
---
apiVersion: v1
kind: Service
metadata:
  name: postgres
  namespace: default
spec:
  type: NodePort
  ports:
  - port: 5432
  selector:
    app: postgres
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: postgres
spec:
  replicas: 1
  selector:
    matchLabels:
      app: postgres
  template:
    metadata:
      labels:
        app: postgres
    spec:
      containers:
        - name: postgres
          image: 'postgres:13.15'
          imagePullPolicy: IfNotPresent
          ports:
            - containerPort: 5432
          envFrom:
            - configMapRef:
                name: postgres-config
EOF

waitForDeployment default "app=postgres"
portForwardAndWait "default" deploy/postgres "5432" "5432"
echo "done setting up postgres"
