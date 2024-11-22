#! /bin/bash
for i in $(seq $RETRIES)
do
	trivy --cache-dir $cache_dir image --download-db-only
	if [ $? -eq 0 ]; then
		break
	fi
	sleep 1
done
