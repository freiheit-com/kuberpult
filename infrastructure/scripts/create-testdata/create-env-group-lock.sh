#!/bin/bash
set -eu
set -o pipefail
set -x

# shellcheck source=ports.sh
source "$(dirname "$0")/ports.sh"
envGroup=prod
lockId=lockIdTest${RANDOM}
lockId=lockIdIntegration0
url="${URL}${FRONTEND_PORT}/environment-groups/${envGroup}/locks/${lockId}"
echo "$url"
useSignature=false
if ${useSignature}
then
  SIGNATURE=$(echo -n "${envGroup}""${lockId}" | gpg --keyring trustedkeys-kuberpult.gpg --local-user kuberpult-kind@example.com --detach --sign --armor)
  json=$(jq -n --arg signature "${SIGNATURE}" --arg message "test env group lock" '$ARGS.named')
else
  json=$(jq -n --arg message "test env group lock" '$ARGS.named')
fi

curl -X PUT "$url" -d "${json}" -H 'Content-Type: application/json'
echo
echo created locks with ID "'""${lockId}""'"



