#!/bin/bash


for i in $(seq 1 10)
do
  echo $i
  file=environments/development/applications/app-alerting-service/locks/ui-"$RANDOM"
  echo "i am a test lock $RANDOM" > "$file"
  git add "$file"
  git commit -m "Add a lock $file"
#  rm "$file"
#  git add "$file"
#  git commit -m "Removed a lock $file"
  git push origin master
  sleep 0.1

done

