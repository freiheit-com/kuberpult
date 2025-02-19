#!/bin/bash
set -e
set -o pipefail

# usage
# ./create-releases.sh base-app-name team number-releases
# Note that this just creates files, it doesn't push in git

BASE_APP_NAME=${1}
applicationOwnerTeam=${2:-sreteam}
NUMBER_RELEASES=${3:-100}

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
index=$((RANDOM % sizeAuthors))
echo "${authors[$index]}"
author="${authors[$index]}"
commit_message_file="$(mktemp "${TMPDIR:-/tmp}/publish.XXXXXX")"
trap "rm -f ""$commit_message_file" INT TERM HUP EXIT


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
index=$((RANDOM % sizeMsgs))
echo $index
echo -e "${msgs[$index]}\n" > "${commit_message_file}"
echo "1: ${msgs[$index]}" >> "${commit_message_file}"
echo "2: ${msgs[$index]}" >> "${commit_message_file}"

ls "${commit_message_file}"

qaEnvs=()
configuration+=("--form" "team=${applicationOwnerTeam}")
search_dir=environments
for env in "$search_dir"/*
do
  env=$(basename "${env}")
  env=$(echo "$env" | awk '{print tolower($0)}')
  if [ "$env" != "testing" ]; then
    qaEnvs+=( "$env" )
  fi
done

FRONTEND_PORT=8081 # see docker-compose.yml

for (( c=1; c<=NUMBER_RELEASES; c++ ))
do
  deployments=$((RANDOM % sizeAuthors))
  n_deployments=$((10 + deployments +1))
  # shellcheck disable=SC2034
  run_release_train=$((RANDOM % n_deployments))
  app=$BASE_APP_NAME-$c
  for (( d=1; d<=n_deployments; d++ ))
  do
    commit_id=$(head -c 20 /dev/urandom | sha1sum | awk '{print $1}') # SHA-1 produces 40-character hashes
    inputs=()
    #inputs+=(--form-string "application=$name")
    inputs+=(--form-string "source_commit_id=$commit_id")
    inputs+=(--form-string "source_author=$author")
    manifests=()
    search_dir=environments

    for env in "$search_dir"/*
    do
      env=$(basename "${env}")
      env=$(echo "$env" | awk '{print tolower($0)}')
      signatureFile=$(mktemp "${TMPDIR:-/tmp}/$env.XXXXXX")
      file=$(mktemp "${x:-/tmp}/$env.XXXXXX")
      # shellcheck disable=SC2034
      randomValue=$(head -c 20 /dev/urandom | sha1sum | awk '{print $1}' | head -c 12)
    cat <<EOF > "${file}"
---
apiVersion: v1
data:
  TNS_ADMIN: /db-config
  TESTDB_SID_LOOKUP_FILENAME: /db-config/locations.json
kind: ConfigMap
metadata:
  name: foo-service-$app-$env
  namespace: md
---
apiVersion: v1
kind: Service
metadata:
  name: foo-service-$app-$env
  namespace: md
spec:
  ports:
  - name: grpc
    port: 50051
    targetPort: 50051
  - name: health
    port: 8080
    targetPort: 8080
  selector:
    app: foo-service-$app-$env
  type: ClusterIP
---
apiVersion: networking.istio.io/v1beta1
kind: VirtualService
metadata:
  name: foo-service-$app-$env
  namespace: md
spec:
  hosts:
  - foo-service-$app-$env
  http:
  - route:
    - destination:
        host: foo-service-$app-$env
        port:
          number: 50051
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
    commit_message=$(head -c 8192 /dev/urandom | base64 | awk '{print $1}' | head -c 128)
    displayVersion=
    if ((RANDOM % 2)); then
      displayVersion=$(( RANDOM % 100)).$(( RANDOM % 100)).$(( RANDOM % 100))
    fi
    index=$((RANDOM % sizeAuthors))
    author="${authors[$index]}"
      time curl http://localhost:${FRONTEND_PORT}/release \
        -H "author-email:${EMAIL}" \
        -H "author-name:${AUTHOR}=" \
        "${inputs[@]}" \
        --form-string "version=$d" \
        --form-string "display_version=${displayVersion}" \
        --form-string "application=$BASE_APP_NAME-$c" \
        --form "source_message=${commit_message}" \
        "${configuration[@]}" \
        "${manifests[@]}"
        echo Created Version "$d" of "$BASE_APP_NAME-$c"
  done
done
