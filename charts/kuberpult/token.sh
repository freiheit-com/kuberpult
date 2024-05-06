#!/bin/bash


UPSTREAM_TOKEN=token

sourcedir="$(dirname $0)"
standard_setup="${FDC_STANDARD_SETUP:-${sourcedir}/../../../../fdc-standard-setup}"
secrets_file="${standard_setup}/secrets/fdc-standard-setup-dev-env-925fe612820f.json"
iap_clientId=$(sops exec-file "${secrets_file}" "jq -r '.client_id' {}")
iap_clientSecret=$(sops exec-file "${secrets_file}" "jq -r '.private_key' {}")
# Authenticate over IAP:
rootDir=$(git rev-parse --show-toplevel)
kuberpultClientId=$(cat "${FDC_STANDARD_SETUP}/infrastructure/terraform/gcp/tools/europe-west1/03_kuberpult/kuberpult-client-id")
kuberpultIapToken=$(sops exec-file --no-fifo "${FDC_STANDARD_SETUP}"/secrets/fdc-standard-setup-dev-env-925fe612820f.json 'GOOGLE_APPLICATION_CREDENTIALS={} bash '"${FDC_STANDARD_SETUP}/infrastructure/scripts/get-iap-token.sh ${kuberpultClientId}")

#TOKEN=$(gcloud auth print-identity-token --impersonate-service-account terraform@fdc-public-docker-registry.iam.gserviceaccount.com)
#TOKEN=$(gcloud auth print-identity-token)
#echo $iap_clientId
TOKEN=$(gcloud auth print-identity-token --impersonate-service-account "106887483619435718021")
curl https://kuberpult.dev.freiheit.systems/dex/token -POST -v \
  -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' \
  --user kuberpult-dex:kuberpult-dex-secret \
  --data-urlencode connector_id=google \
  --data-urlencode response_type=id_token \
  --data-urlencode grant_type=authorization_code \
  --data-urlencode scope="openid" \
  --data-urlencode requested_token_type=urn:ietf:params:oauth:token-type:access_token \
  --data-urlencode subject_token=$kuberpultIapToken \
  --data-urlencode subject_token_type=urn:ietf:params:oauth:token-type:access_token
