#!/usr/bin/env bash
set -euo pipefail

echo "getting frontend-service logs..."
echo "************************************************************"
kubectl logs -l "app=kuberpult-frontend-service"
echo "************************************************************"
echo "************************************************************"

echo "getting cd-service logs..."
echo "************************************************************"
kubectl logs -l "app=kuberpult-cd-service"
echo "************************************************************"
echo "************************************************************"

echo "getting rollout-service logs..."
echo "************************************************************"
kubectl logs -l "app=kuberpult-rollout-service"
echo "************************************************************"
echo "************************************************************"

echo "getting export-service logs..."
echo "************************************************************"
kubectl logs -l "app=kuberpult-manifest-repo-export-service"
echo "************************************************************"
echo "************************************************************"

exit 1