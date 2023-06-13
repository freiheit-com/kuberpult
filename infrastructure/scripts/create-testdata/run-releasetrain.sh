#!/bin/bash
set -eu
set -o pipefail
#set -x

env=${1:-fakeprod-ca}
if test "$#" -eq 2; then
  url="http://localhost:8081/environments/${env}/releasetrain?""$2"
else
  url="http://localhost:8081/environments/${env}/releasetrain"
fi


curl -X PUT "$url"
echo



