#!/bin/bash

set -eu # protection
#set -x # enable debug logs

echo --------------------Versions------------------------------
for env in environments/* # $(find environments  -maxdepth 1 -mindepth 1 -type d)
do
  for app in "$env"/applications/*
  do
    # If $app has version
    echo Application Version: "$app"
    if [ -d "$app"/version ]; then
      git log -1 --format="%ad" -- "$app"/version > "$app"/deploy_date
    fi
  done
done
echo --------------------Versions------------------------------
