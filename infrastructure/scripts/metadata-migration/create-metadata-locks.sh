#!/usr/bin/env bash

set -eou pipefail # protection
#set -x # enable debug logs

for env in environments/* # $(find environments  -maxdepth 1 -mindepth 1 -type d)
do
  echo Looking in: "$env"

  echo -----------------Application Locks------------------------
  for app in "$env"/applications/*
  do
    # If $app has locks
    if [ -d "$app"/locks ]; then
      # shellcheck disable=SC2012
      echo Adding metadata to "$(ls "$app"/locks | wc -l)" locks in "$app"
      find "$app"/locks  -maxdepth 1 -mindepth 1 -type f -print0 | while IFS= read -r -d '' lock
      do
        echo Lock ID: "$(basename "$lock")" - Lock Message: "$(cat "$lock")"
        date=$(git log -1 --date=iso-strict --format="%ad" -- "$lock")
        name=$(git log -1 --format="%an" -- "$lock")
        email=$(git log -1 --format="%ae" -- "$lock")
        msg=$(cat "$lock")
        rm "$lock"
        mkdir -p "$lock"
        echo "$date" > "$lock"/created_at
        echo "$name" > "$lock"/created_by_name
        echo "$email" > "$lock"/created_by_email
        echo "$msg" > "$lock"/message
      done
    fi
  done
  echo -----------------Application Locks------------------------

  echo -----------------Environment Locks------------------------
  # If $env has locks
  if [ -d "$env"/locks ]; then
    # shellcheck disable=SC2012
    echo Adding metadata to "$(ls "$env"/locks | wc -l)" locks in "$env"
    find "$env"/locks  -maxdepth 1 -mindepth 1 -type f -print0 | while IFS= read -r -d '' lock
    do
      echo Lock ID: "$(basename "$lock")" - Lock Message: "$(cat "$lock")"
      date=$(git log -1 --date=iso-strict --format="%ad" -- "$lock")
      name=$(git log -1 --format="%an" -- "$lock")
      email=$(git log -1 --format="%ae" -- "$lock")
      msg=$(cat "$lock")
      rm "$lock"
      mkdir -p "$lock"
      echo "$date" > "$lock"/created_at
      echo "$name" > "$lock"/created_by_name
      echo "$email" > "$lock"/created_by_email
      echo "$msg" > "$lock"/message
    done
  fi
  echo -----------------Environment Locks------------------------
done
