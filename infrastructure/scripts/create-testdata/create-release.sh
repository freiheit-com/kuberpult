#!/bin/bash
set -eu
set -o pipefail
#set -x

# usage
# ./create-release.sh my-service-name [my-team-name]
# Note that this just creates files, it doesn't push in git

name=${1}
applicationOwnerTeam=${2:-sreteam}
commit_id=$(tr -dc a-f0-9 </dev/urandom | head -c 12 ; echo '')
author="The Author"
commit_message_file="$(mktemp "${TMPDIR:-/tmp}/publish.XXXXXX")"
trap "rm -f ""$commit_message_file" INT TERM HUP EXIT
echo "This is the commit $commit_id" > "${commit_message_file}"
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


