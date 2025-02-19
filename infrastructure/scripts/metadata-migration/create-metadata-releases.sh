#!/usr/bin/env bash

set -eou pipefail # protection
#set -x # enable debug logs

echo --------------------Releases------------------------------
for app in applications/* # $(find applications  -maxdepth 1 -mindepth 1 -type d)
do
  # shellcheck disable=SC2012
  echo Adding metadata to "$(ls "$app"/releases | wc -l)" releases in "$app"
  find "$app"/releases  -maxdepth 1 -mindepth 1 -type d -print0 | while IFS= read -r -d '' release
  do
    echo Release: "$(basename "$release")"
    git log -1 --date=iso-strict --format="%ad" -- "$release" > "$release"/created_at
  done
done
echo --------------------Releases------------------------------
