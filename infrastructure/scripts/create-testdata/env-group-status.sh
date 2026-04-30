#!/bin/bash
set -eu
set -o pipefail
set -x

FRONTEND_PORT=${KUBERPULT_PORT_FRONTEND_HTTP:-8081}
envGroup="${1:-development}"
url="http://localhost:${FRONTEND_PORT}/environment-groups/${envGroup}/rollout-status"
useSignature=false
if ${useSignature}
then
  SIGNATURE=$(echo -n "${envGroup}" | gpg --keyring trustedkeys-kuberpult.gpg --local-user kuberpult-kind@example.com --detach --sign --armor)
  json=$(jq -n --arg signature "${SIGNATURE}" '$ARGS.named')
else
  json="{}"
fi

curl -X POST "$url" -d "${json}" -H 'Content-Type: application/json'
