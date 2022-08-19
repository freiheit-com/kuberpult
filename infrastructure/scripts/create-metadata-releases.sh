#!/bin/bash

set -eu # protection
#set -x # enable debug logs

for env in environments/* # $(find environments  -maxdepth 1 -mindepth 1 -type d)
do
  echo Looking in: "$env"

  echo -----------------Application Locks------------------------
  for app in "$env"/applications/*
  do
    # If $app has locks
    if [ -d "$app"/locks ]; then
      echo Adding metadata to "$(ls "$app"/locks | wc -l)" locks in "$app"
      find "$app"/locks  -maxdepth 1 -mindepth 1 -type f -print0 | while IFS= read -r -d '' lock
      do
        echo Lock ID: "$(basename "$lock")" - Lock Message: "$(cat "$lock")"
        LOCK_DIR="$(dirname "$lock")"/lock_"$(basename "$lock")"
        mkdir -p "$LOCK_DIR"
        git log -1 --format="%ad" -- "$lock" > "$LOCK_DIR"/author_date
        git log -1 --format="%an" -- "$lock" > "$LOCK_DIR"/author_name
        git log -1 --format="%ae" -- "$lock" > "$LOCK_DIR"/author_email
        basename "$lock" > "$LOCK_DIR"/lock_id
        cat "$lock" > "$LOCK_DIR"/lock_message
      done
    fi
  done
  echo -----------------Application Locks------------------------

  echo -----------------Environment Locks------------------------
  # If $env has locks
  if [ -d "$env"/locks ]; then
    echo Adding metadata to "$(ls "$env"/locks | wc -l)" locks in "$env"
    find "$env"/locks  -maxdepth 1 -mindepth 1 -type f -print0 | while IFS= read -r -d '' lock
    do
      echo Lock ID: "$(basename "$lock")" - Lock Message: "$(cat "$lock")"
      LOCK_DIR="$(dirname "$lock")"/lock_"$(basename "$lock")"
      mkdir -p "$LOCK_DIR"
      git log -1 --format="%ad" -- "$lock" > "$LOCK_DIR"/author_date
      git log -1 --format="%an" -- "$lock" > "$LOCK_DIR"/author_name
      git log -1 --format="%ae" -- "$lock" > "$LOCK_DIR"/author_email
      basename "$lock" > "$LOCK_DIR"/lock_id
      cat "$lock" > "$LOCK_DIR"/lock_message
    done
  fi
  echo -----------------Environment Locks------------------------
done