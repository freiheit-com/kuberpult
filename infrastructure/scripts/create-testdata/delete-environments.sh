#!/bin/bash
set -eu
set -o pipefail
# usage
# ./create-environments.sh [path/to/envs]
# Note that this just creates files, it doesn't push in git

cd "$(dirname "$0")"

# To export the keyring:
# gpg --output pgpRing-local-public.pgp --armor --expor

# Configure to use that keyring in frontend-service:
# KUBERPULT_PGP_KEY_RING_PATH=/kp/kuberpult/pgpRing-local-public.pgp

# To generate the signature:
# echo -n 'fakeprod-ca' | gpg --local-user GPG_USER_EMAIL --detach --sign --armor  > ./fakeprod-ca.yaml.sig

FRONTEND_PORT=8081 # see docker-compose.yml
env=${1}
curl  -f -X DELETE  \
    --form signature=@"$env".yaml.sig \
    http://localhost:${FRONTEND_PORT}/api/environments/"${env}" -v

echo # curl sometimes does not print a trailing \n
