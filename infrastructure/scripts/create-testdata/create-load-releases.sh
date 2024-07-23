#!/bin/bash
set -eu
set -o pipefail

# usage
# ./create-release.sh my-service-name [my-team-name]
# Note that this just creates files, it doesn't push in git

BASE_APP_NAME=${1}
applicationOwnerTeam=${2:-sreteam}
TEMPLATED_ENV_NAME=${3-loadTesting}
NUMBER_TEMPLATED_ENVS=${4-50}
NUMBER_RELEASES=${5:-100}

# 40 is the length of a full git commit hash.
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
echo -e "${msgs[$index]}\n" > "${commit_message_file}"
echo "1: ${msgs[$index]}" >> "${commit_message_file}"
echo "2: ${msgs[$index]}" >> "${commit_message_file}"

ls "${commit_message_file}"

release_version=''
case "${RELEASE_VERSION:-}" in
	*[!0-9]*) echo "Please set the env variable RELEASE_VERSION to a number"; exit 1;;
	*) release_version='--form-string '"version=${RELEASE_VERSION:-}";;
esac

echo "release version:" "${release_version}"

configuration=()
configuration+=("--form" "team=${applicationOwnerTeam}")

FRONTEND_PORT=8081 # see docker-compose.yml

for (( c=1; c<=NUMBER_RELEASES; c++ ))
do
  deployments=$(($RANDOM % $sizeAuthors))
  n_deployments=$(($deployments +1))

  for (( d=1; d<=n_deployments; d++ ))
  do
    commit_id=$(head -c 20 /dev/urandom | sha1sum | awk '{print $1}') # SHA-1 produces 40-character hashes
    inputs=()
    #inputs+=(--form-string "application=$name")
    inputs+=(--form-string "source_commit_id=$commit_id")
    inputs+=(--form-string "source_author=$author")
    manifests=()
    for env in development development2 staging fakeprod-de fakeprod-ca
    do
      file=$(mktemp "${TMPDIR:-/tmp}/$env.XXXXXX")
      signatureFile=$(mktemp "${TMPDIR:-/tmp}/$env.XXXXXX")
        randomValue=$(head -c 20 /dev/urandom | sha1sum | awk '{print $1}' | head -c 12)
cat <<EOF > "${file}"
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: ${name-dummy-config-map}
  namespace: "$env"
data:
  key: value
  random: "${randomValue}"
---
EOF
      manifests+=("--form" "manifests[${env}]=@${file}")
      gpg  --keyring trustedkeys-kuberpult.gpg --local-user kuberpult-kind@example.com --detach --sign --armor < "${file}" > "${signatureFile}"
      manifests+=("--form" "signatures[${env}]=@${signatureFile}")
    done

    for (( env_n=1; env_n<=NUMBER_TEMPLATED_ENVS; env_n++ ))
    do
      env="${TEMPLATED_ENV_NAME}-${env_n}"
      file=$(mktemp "${TMPDIR:-/tmp}/$env.XXXXXX")
      signatureFile=$(mktemp "${TMPDIR:-/tmp}/$env.XXXXXX")
      #randomValue=$(head -c 20 /dev/urandom | sha1sum | awk '{print $1}' | head -c 12)
      randomValue=$(head -c 8192 /dev/urandom | base64 | awk '{print $1}' | head -c 8192)
cat <<EOF > "${file}"
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: ${name-dummy-config-map}
  namespace: "$env"
data:
  key: value
  random: "${randomValue}"
---
EOF
      manifests+=("--form" "manifests[${env}]=@${file}")
      gpg  --keyring trustedkeys-kuberpult.gpg --local-user kuberpult-kind@example.com --detach --sign --armor < "${file}" > "${signatureFile}"
      manifests+=("--form" "signatures[${env}]=@${signatureFile}")
    done
    if [[ $(uname -o) == Darwin ]];
    then
      EMAIL=$(echo -n "script-user@example.com" | base64 -b 0)
      AUTHOR=$(echo -n "script-user" | base64 -b 0)
    else
      EMAIL=$(echo -n "script-user@example.com" | base64 -w 0)
      AUTHOR=$(echo -n "script-user" | base64 -w 0)
    fi
      curl http://localhost:${FRONTEND_PORT}/release \
        -H "author-email:${EMAIL}" \
        -H "author-name:${AUTHOR}=" \
        "${inputs[@]}" \
        --form-string "version=$d" \
        --form-string "display_version=${displayVersion}" \
        --form-string "application=$BASE_APP_NAME-$c" \
        --form "source_message=<${commit_message_file}" \
        "${configuration[@]}" \
        "${manifests[@]}"
        echo # curl sometimes does not print a trailing \n
        echo Created Version "$d" of "$BASE_APP_NAME-$c"
  done
done
