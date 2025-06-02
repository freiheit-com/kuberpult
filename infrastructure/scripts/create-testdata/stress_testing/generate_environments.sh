#!/bin/bash

set -e
set -o pipefail

BASE_ENV_NAME=${1:-qa}
NUMBER_ENVS=${2:-32}
UPSTREAM_NAME=${3-testing}

mkdir -p environments/"$UPSTREAM_NAME"
NUMBER_ENVS=$((NUMBER_ENVS-1))
touch environments/"$UPSTREAM_NAME"/config.json
cat <<EOF > "environments/${UPSTREAM_NAME}/config.json"
{
  "argocd": {
    "destination": {
      "server": "https://kubernetes.default.svc",
      "namespace": "*"
    },
    "applicationAnnotations": {
      "notifications.argoproj.io/subscribe.on-degraded.teams":"",
      "notifications.argoproj.io/subscribe.on-sync-failed.teams":""
    },
    "accessList": [
      {
        "group": "*",
        "kind": "ClusterSecretStore"
      },
      {
        "group": "*",
        "kind": "ClusterIssuer"
      }
    ],
    "ignoreDifferences": [
      {
        "group": "apps",
        "kind": "Deployment",
        "jsonPointers": [
          "/spec/replicas"
        ]
      }
    ]
  },
  "upstream": {
    "latest": true
  },
  "environment_group": "${UPSTREAM_NAME}"
}
EOF

while IFS= read -r line ; do
    if [ "$NUMBER_ENVS" -lt 0 ]; then
      break
    fi
    mkdir -p environments/"$BASE_ENV_NAME"-"$line"
    NUMBER_ENVS=$((NUMBER_ENVS-1))
    touch environments/"$BASE_ENV_NAME"-"$line"/config.json
cat <<EOF > "environments/${BASE_ENV_NAME}-${line}/config.json"
{
  "argocd": {
    "destination": {
      "server": "https://kubernetes.default.svc",
      "namespace": "*"
    },
    "applicationAnnotations": {
      "notifications.argoproj.io/subscribe.on-degraded.teams":"",
      "notifications.argoproj.io/subscribe.on-sync-failed.teams":""
    },
    "accessList": [
      {
        "group": "*",
        "kind": "ClusterSecretStore"
      },
      {
        "group": "*",
        "kind": "ClusterIssuer"
      }
    ],
    "ignoreDifferences": [
      {
        "group": "apps",
        "kind": "Deployment",
        "jsonPointers": [
          "/spec/replicas"
        ]
      }
    ]
  },
  "upstream": {
    "environment": "testing"
  },
  "environmentGroup": "${BASE_ENV_NAME}"
}
EOF
done < country_codes.csv
/bin/bash ./../create-environments.sh stress_testing/environments