#!/bin/bash
set -eu
set -o pipefail
#set -x

# usage
# ./create-release.sh my-service-name [my-team-name]
# Note that this just creates files, it doesn't push in git

name=${1}
applicationOwnerTeam=${2:-sreteam}
prev=${3:-""}
next=${4:-""}

# 40 is the length of a full git commit hash.
commit_id=$(LC_CTYPE=C tr -dc a-f0-9 </dev/urandom | head -c 40 ; echo '')
authors[0]="urbansky"
authors[1]="Medo"
authors[2]="Hannes"
authors[3]="Mouhsen"
authors[4]="Tamer"
authors[5]="Ahmed"
authors[6]="João"
authors[7]="Leandro"
sizeAuthors=${#authors[@]}
index=$(($RANDOM % $sizeAuthors))
echo ${authors[$index]}
author="${authors[$index]}"
commit_message_file="$(mktemp "${TMPDIR:-/tmp}/publish.XXXXXX")"
trap "rm -f ""$commit_message_file" INT TERM HUP EXIT
displayVersion=
if ((RANDOM % 2)); then
  displayVersion=$(( $RANDOM % 100)).$(( $RANDOM % 100)).$(( $RANDOM % 100))
fi

msgs[0]="Added new eslint rule"
msgs[1]="Fix annotations in helm templates"
msgs[2]="Improve performance with gitLib2"
msgs[3]="Add colors to new UI"
msgs[4]="Release trains for env groups"
msgs[5]="Fix whitespace in ReleaseDialog"
msgs[6]="Add Snackbar Notifications"
msgs[7]="Rephrase ReleaseDialog text"
msgs[8]="Change renovate schedule"
msgs[9]="Fix bug in distanceToUpstream calculation"
msgs[10]="Allow deleting locks on locks page"
sizeMsgs=${#msgs[@]}
index=$(($RANDOM % $sizeMsgs))
echo $index
echo "${msgs[$index]}" > "${commit_message_file}"

ls "${commit_message_file}"

release_version=''
case "${RELEASE_VERSION:-}" in
	'') echo "No RELEASE_VERSION set. Using non-idempotent versioning scheme";;
	*[!0-9]*) echo "Please set the env variable RELEASE_VERSION to a number"; exit 1;;
	*) release_version='--form-string '"version=${RELEASE_VERSION:-}";;
esac

echo "release version:" "${release_version}"

configuration=()
configuration+=("--form" "team=${applicationOwnerTeam}")

manifests=()
for env in development development2 staging fakeprod-de fakeprod-ca
do
  file=$(mktemp "${TMPDIR:-/tmp}/$env.XXXXXX")
  signatureFile=$(mktemp "${TMPDIR:-/tmp}/$env.XXXXXX")
  randomValue=$(LC_CTYPE=C tr -dc a-f0-9 </dev/urandom | head -c 12 ; echo '')
cat <<EOF > "${file}"
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: $name-dummy-config-map
  namespace: "$env"
data:
  key: value
  random: "${randomValue}"
---
EOF
  echo "wrote file ${file}"
  manifests+=("--form" "manifests[${env}]=@${file}")
  gpg  --keyring trustedkeys-kuberpult.gpg --local-user kuberpult-kind@example.com --detach --sign --armor < "${file}" > "${signatureFile}"
  manifests+=("--form" "signatures[${env}]=@${signatureFile}")
done
echo commit id: "${commit_id}"

FRONTEND_PORT=8081 # see docker-compose.yml

if [[ $(uname -o) == Darwin ]];
then
  EMAIL=$(echo -n "script-user@example.com" | base64 -b0)
  AUTHOR=$(echo -n "script-user" | base64 -b0)
else
  EMAIL=$(echo -n "script-user@example.com" | base64 -w 0)
  AUTHOR=$(echo -n "script-user" | base64 -w 0)
fi

curl http://localhost:${FRONTEND_PORT}/release \
  -H "author-email:${EMAIL}" \
  -H "author-name:${AUTHOR}=" \
  --form-string "application=$name" \
  --form-string "source_commit_id=${commit_id}" \
  --form-string "source_author=${author}" \
  --form-string "previous_commit_id=${prev}" \
  --form-string "next_commit_id=${next}" \
  ${release_version} \
  --form-string "display_version=${displayVersion}" \
  --form "source_message=<${commit_message_file}" \
  "${configuration[@]}" \
  "${manifests[@]}" -v

echo # curl sometimes does not print a trailing \n

