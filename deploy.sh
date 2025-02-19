#!/bin/bash

# Runs through all manifests and applies them in kubernetes
# Expects a directory structure of "environment/$environment/applications/$appname/...
# Run with "environment" directory as first parameter and "namespace" as second parameter and server url as third.
# The output is intended to be piped into kubectl, e.g.
# ./deploy.sh my-dir development "https://8.8.8.8" | kubectl apply -f -

set -eup

ROOT="$1"
NAMESPACE="$2"
URL="$3"

cd "$ROOT"
for env in *; do
  if test -d "$env"/applications; then
    cd "$env"/applications
    for appname in *; do
      if test -d "$appname"; then
        echo "Applying manifest for $appname ..." > /dev/stderr
        manifestsDir="environments/$env/applications/$appname/manifests"
        cat << EOF
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: "$NAMESPACE-$appname"
  namespace: tools
spec:
  destination:
    name: ""
    namespace: $NAMESPACE
    server: "$URL"
  source:
    path: $manifestsDir
    repoURL: "git@github.com:freiheit-com/nmww-manifests.git"
    targetRevision: kuberpult
  project: default
  syncPolicy:
    automated:
      prune: false
      selfHeal: false
---
EOF
      fi
    done
  fi
done

echo "$0 done" > /dev/stderr

exit 0
