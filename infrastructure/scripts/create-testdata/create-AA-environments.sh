#!/bin/bash
set -eu
set -o pipefail
# usage
# ./create-environments.sh [path/to/envs]
# Note that this just creates files, it doesn't push in git

FRONTEND_PORT=8081 # see docker-compose.yml

  DATA=$(cat <<YAML
{

  "argoConfigs": {
    "commonEnvPrefix": "aa",
    "configs": [
      {
        "destination": {
          "server": "https://kubernetes.default.svc",
          "namespace": "*"
        },
        "application_annotations": {
          "notifications.argoproj.io/subscribe.on-degraded.teams":"",
          "notifications.argoproj.io/subscribe.on-sync-failed.teams":""
        },
        "access_list": [
          {
            "group": "*",
            "kind": "ClusterSecretStore"
          },
          {
            "group": "*",
            "kind": "ClusterIssuer"
          }
        ],
        "ignore_differences": [
          {
            "group": "apps",
            "kind": "Deployment",
            "jsonPointers": [
              "/spec/replicas"
            ]
          }
        ],
        "concreteEnvName": "dev-2"
      },
      {
        "destination": {
          "server": "https://kubernetes.default.svc",
          "namespace": "*"
        },
        "application_annotations": {
          "notifications.argoproj.io/subscribe.on-degraded.teams":"",
          "notifications.argoproj.io/subscribe.on-sync-failed.teams":""
        },
        "access_list": [
          {
            "group": "*",
            "kind": "ClusterSecretStore"
          },
          {
            "group": "*",
            "kind": "ClusterIssuer"
          }
        ],
        "ignore_differences": [
          {
            "group": "apps",
            "kind": "Deployment",
            "jsonPointers": [
              "/spec/replicas"
            ]
          }
        ],
        "concreteEnvName": "dev-1"
      }
    ]
  },
  "upstream": {
    "environment": "development2"
  },
  "environmentGroup": "staging"
}
YAML
)
curl -X POST -H "multipart/form-data" \
      --form-string "config=${DATA}" \
       http://localhost:${FRONTEND_PORT}/api/environments/testing

echo # curl sometimes does not print a trailing \n

