#!/bin/sh

i=0
status=0
while [ "$i" -lt $1 ]; do
	trivy --cache-dir $2 image --download-db-only
	status=$?
	if [ $status -eq 0 ]; then
		break
	fi
	sleep 2
	i=$(( i + 1 ))
done

exit $status
