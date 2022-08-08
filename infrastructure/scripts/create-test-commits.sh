#!/bin/bash


for i in $(seq 1 1000)
do
  echo $i
  file=environments/development/applications/app-alerting-service/locks/ui-"$RANDOM"
  echo "i am a test lock $RANDOM" > "$file"
  git add "$file"
  git ci -m "Add a lock $file"
  rm "$file"
  git add "$file"
  git ci -m "Removed a lock $file"
  git push origin master
  sleep 0.1

done

