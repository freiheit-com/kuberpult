#! /bin/bash
for i in $(seq $1)
do
	trivy --cache-dir $cache_dir image --download-db-only
	if [ $? -eq 0 ]; then
		break
	fi
	sleep 1
done
