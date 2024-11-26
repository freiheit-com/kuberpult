#!/bin/sh

i=0
status=1
while [ "$i" -lt $1 ]; do
	oras cp ghcr.io/aquasecurity/trivy-db:2 europe-west3-docker.pkg.dev/fdc-public-docker-registry/kuberpult/aquasecurity/trivy-db:2
	# trivy --cache-dir $2 image --download-db-only
	status=$?
	if [ $status -eq 0 ]; then
		break
	fi
	sleep 2
	i=$(( i + 1 ))
done
exit $status

