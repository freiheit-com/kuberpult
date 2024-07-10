#!/usr/bin/env bash
set -eu
set -o pipefail

minikube image load europe-west3-docker.pkg.dev/fdc-public-docker-registry/kuberpult/git-ssh:1.1.1
minikube image load postgres:local
minikube image load $REGISTRY_URI/kuberpult-cd-service:$VERSION
minikube image load $REGISTRY_URI/kuberpult-frontend-service:$VERSION
minikube image load $REGISTRY_URI/kuberpult-rollout-service:$VERSION
minikube image load $REGISTRY_URI/kuberpult-manifest-repo-export-service:$VERSION
minikube image load $ARGOCD_IMAGE_URI
minikube image load $DEX_IMAGE_URI
minikube image load $CLOUDSQL_PROXY_IMAGE_URI
minikube image load $REDIS_IMAGE_URI