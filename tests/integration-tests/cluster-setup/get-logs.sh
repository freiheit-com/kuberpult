#!/usr/bin/env bash
set -euo pipefail

sleep 3
echo "getting frontend-service logs..."
echo "************************************************************"
kubectl logs -l "app=kuberpult-frontend-service" --tail -1
echo "************************************************************"
echo "************************************************************"

echo "getting cd-service logs..."
echo "************************************************************"
kubectl logs -l "app=kuberpult-cd-service" --tail -1
echo "************************************************************"
echo "************************************************************"

echo "getting rollout-service logs..."
echo "************************************************************"
kubectl logs -l "app=kuberpult-rollout-service" --tail -1
echo "************************************************************"
echo "************************************************************"

echo "getting export-service logs..."
echo "************************************************************"
kubectl logs -l "app=kuberpult-manifest-repo-export-service" --tail -1
echo "************************************************************"
echo "************************************************************"

exit 1