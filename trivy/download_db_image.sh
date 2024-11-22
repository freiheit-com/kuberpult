#! /bin/bash
for i in $(seq $1)
do
	trivy --cache-dir $2 image --download-db-only
	if [ $? -eq 0 ]; then
		break
	fi
	sleep 1
done