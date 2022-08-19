#!/bin/bash

set -eu # protection
#set -x # enable debug logs

echo --------------------Releases------------------------------
for app in applications/* # $(find applications  -maxdepth 1 -mindepth 1 -type d)
do
  echo Adding metadata to "$(ls "$app"/releases | wc -l)" releases in "$app"
  for release in "$app"/releases/*
  do
    echo Release: "$(basename "$release")"
    git log -1 --format="%ad" -- "$release" > "$release"/release_date
  done
done
echo --------------------Releases------------------------------
