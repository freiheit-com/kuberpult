#!/bin/bash
set -eu
set -o pipefail

# usage
# ./create-release.sh my-service-name [my-team-name]
# Note that this just creates files, it doesn't push in git

name=${1}
applicationOwnerTeam=${2:-sreteam}
argoBracket=${3:-""}

function debug() {
    echo "$@" > /dev/stderr
}

# 40 is the length of a full git commit hash.
commit_id=$(head -c 20 /dev/urandom | sha1sum | awk '{print $1}') # SHA-1 produces 40-character hashes
debug "commit id is: ${commit_id}"
prev_commit_id=$(head -c 20 /dev/urandom | sha1sum | awk '{print $1}') # SHA-1 produces 40-character hashes
debug "prev_commit_id is: ${prev_commit_id}"
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
author="${authors[$index]}"
commit_message_file="$(mktemp "${TMPDIR:-/tmp}/publish.XXXXXX")"
trap "rm -f ""$commit_message_file" INT TERM HUP EXIT
displayVersion=
if ((RANDOM % 2)); then
  displayVersion=$(( RANDOM % 100)).$(( RANDOM % 100)).$(( RANDOM % 100))
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
index=$((RANDOM % sizeMsgs))
echo -e "${msgs[$index]}\n" > "${commit_message_file}"
echo "1: ${msgs[$index]}" >> "${commit_message_file}"
echo "2: ${msgs[$index]}" >> "${commit_message_file}"


release_version=()
case "${RELEASE_VERSION:-}" in
	*[!0-9]*) echo "Please set the env variable RELEASE_VERSION to a number"; exit 1;;
	*) release_version+=('--form-string' "version=${RELEASE_VERSION:-}");;
esac

rev=${REVISION:-"0"}
revision=('--form-string' "revision=${rev}")

configuration=()
configuration+=("--form" "team=${applicationOwnerTeam}")
if [ -z "${argoBracket}" ]; then
  echo "skipping argoBracket"
else
  configuration+=("--form" "experimentalArgoBracket=${argoBracket}")
fi

manifests=()
for env in development development2 staging fakeprod-de fakeprod-ca aa-test
do
  file=$(mktemp "${TMPDIR:-/tmp}/$env.XXXXXX")
  randomValue=$(head -c 20 /dev/urandom | sha1sum | awk '{print $1}' | head -c 12)
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
  releaseVersion: "${release_version[@]}"
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: $name-sleep-deployment
  namespace: "$env"
spec:
  replicas: 1
  selector:
    matchLabels:
      app: $name-sleep
  template:
    metadata:
      labels:
        app: $name-sleep
    spec:
      containers:
      - name: $name-sleep-container
        image: alpine:latest
        # We use 'trap' so the container handles termination signals gracefully
        command: ["/bin/sh", "-c", "trap 'exit 0' SIGTERM; while true; do sleep 30; done"]
        # Readiness Probe: Tells K8s when the pod is ready to bridge traffic
        readinessProbe:
          exec:
            command:
            - ls
            - /
          initialDelaySeconds: 5
          periodSeconds: 5
        # Liveness Probe: Tells K8s if the container needs a restart
        livenessProbe:
          exec:
            command:
            - ps
            - aux
          initialDelaySeconds: 10
          periodSeconds: 10
        resources:
          limits:
            cpu: "100m"
            memory: "64Mi"
          requests:
            cpu: "10m"
            memory: "32Mi"
---
EOF
  manifests+=("--form" "manifests[${env}]=@${file}")
done

echo "manifest is in $file"

FRONTEND_PORT=8081 # see docker-compose.yml

if [[ $(uname -o) == Darwin ]];
then
  EMAIL=$(echo -n "script-user@example.com" | base64 -b 0)
  AUTHOR=$(echo -n "script-user" | base64 -b 0)
else
  EMAIL=$(echo -n "script-user@example.com" | base64 -w 0)
  AUTHOR=$(echo -n "script-user" | base64 -w 0)
fi

inputs=()
inputs+=(--form-string "application=$name")
inputs+=(--form-string "source_commit_id=$commit_id")
inputs+=(--form-string "source_author=$author")
inputs+=(--form-string "previous_commit_id=${prev_commit_id}")

curl http://localhost:${FRONTEND_PORT}/api/release \
  -H "author-email:${EMAIL}" \
  -H "author-name:${AUTHOR}=" \
  "${inputs[@]}" \
  "${release_version[@]}" \
  "${revision[@]}" \
  --form-string "display_version=${displayVersion}" \
  --form "source_message=<${commit_message_file}" \
  "${configuration[@]}" \
  "${manifests[@]}"

echo # curl sometimes does not print a trailing \n