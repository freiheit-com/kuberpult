#!/bin/bash
set -eu
set -o pipefail
#set -x

# usage
# ./create-release.sh my-service-name [my-team-name]
# Note that this just creates files, it doesn't push in git

name=${1}
applicationOwnerTeam=${2:-sreteam}
commit_id=$(LC_CTYPE=C tr -dc a-f0-9 </dev/urandom | head -c 12 ; echo '')
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
echo ${msgs[$index]}" "$commit_id > "${commit_message_file}"

ls ${commit_message_file}

release_version=()
configuration=()
configuration+=("--form" "team=${applicationOwnerTeam}")

manifests=()
for env in development development2 staging fakeprod-de fakeprod-ca
do
  file=$(mktemp "${TMPDIR:-/tmp}/$env.XXXXXX")
  echo "---" > ${file}
  echo "wrote file ${file}"
  manifests+=("--form" "manifests[${env}]=@${file}")
done
echo commit id: ${commit_id}


curl http://localhost:8081/release \
  --form-string "application=$name" \
  --form-string "source_commit_id=${commit_id}" \
  --form-string "source_author=${author}" \
  --form "source_message=<${commit_message_file}" \
  "${configuration[@]}" \
  "${manifests[@]}"


