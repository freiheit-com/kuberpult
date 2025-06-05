# This file is part of kuberpult.

# Kuberpult is free software: you can redistribute it and/or modify
# it under the terms of the Expat(MIT) License as published by
# the Free Software Foundation.

# Kuberpult is distributed in the hope that it will be useful,
# but WITHOUT ANY WARRANTY; without even the implied warranty of
# MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
# MIT License for more details.

# You should have received a copy of the MIT License
# along with kuberpult. If not, see <https://directory.fsf.org/wiki/License:Expat>.

# Copyright freiheit.com
.PHONY: build-base-image push-base-image scan-service-pr scan-service scan-all-pr

IMAGE_REGISTRY:=europe-west3-docker.pkg.dev/fdc-public-docker-registry/kuberpult
TRIVY_IMAGE_REGISTRY:=$(IMAGE_REGISTRY)/trivy
TRIVY_DB_REPO:=$(IMAGE_REGISTRY)/aquasecurity/trivy-db
VERSION?=$(shell git describe --always --long --tags || echo 0.0.1)
TRIVY_IMAGE_URL:=$(TRIVY_IMAGE_REGISTRY):$(VERSION)
SERVICE?=cd-service
GCP_CREDENTIALS_PATH?=$(HOME)/.config/gcloud/application_default_credentials.json
KUBERPULT_SERVICE_IMAGE=$(IMAGE_REGISTRY)/kuberpult-$(SERVICE):$(VERSION)
KUBERPULT_SERVICE_REPORT_HTML=kuberpult-$(SERVICE).html
KUBERPULT_SERVICE_REPORT_DIR="kuberpult-$(SERVICE)-reports"

build-base-image:
	docker build . -t $(TRIVY_IMAGE_URL)

push-base-image:
	docker push $(TRIVY_IMAGE_URL)

scan-service-pr:
	docker run --rm -e TRIVY_USERNAME="" -e GOOGLE_APPLICATION_CREDENTIALS="/root/credentials.json" -v "$(GCP_CREDENTIALS_PATH)":/root/credentials.json:ro $(TRIVY_IMAGE_URL) \
		trivy --db-repository $(TRIVY_DB_REPO)  --cache-dir /tmp/trivy_cache --severity HIGH,CRITICAL --exit-code 1 image "$(KUBERPULT_SERVICE_IMAGE)"

scan-service:
	mkdir -p $(KUBERPULT_SERVICE_REPORT_DIR)
	touch ./$(KUBERPULT_SERVICE_REPORT_DIR)/$(KUBERPULT_SERVICE_REPORT_HTML)
	docker run --rm -e TRIVY_USERNAME="" -e GOOGLE_APPLICATION_CREDENTIALS="/root/credentials.json" -v "$(GCP_CREDENTIALS_PATH):/root/.config/gcloud/application_default_credentials.json" \
	 -v "./$(KUBERPULT_SERVICE_REPORT_DIR)/$(KUBERPULT_SERVICE_REPORT_HTML):/trivy/$(KUBERPULT_SERVICE_REPORT_HTML)" $(TRIVY_IMAGE_URL) \
		trivy --db-repository $(TRIVY_DB_REPO) --cache-dir /tmp/trivy_cache image "$(KUBERPULT_SERVICE_IMAGE)" -f template --template "@templates/html.tpl" -o $(KUBERPULT_SERVICE_REPORT_HTML)
	tar -czf "$(KUBERPULT_SERVICE_REPORT_DIR).tar.gz" $(KUBERPULT_SERVICE_REPORT_DIR)

scan-all-pr:
	@for service in $(wildcard ../services/*); do \
		SERVICE=$$(basename $$service) $(MAKE) scan-service-pr; \
	done

scan-all:
	@for service in $(wildcard ../services/*); do \
		SERVICE=$$(basename $$service) $(MAKE) scan-service; \
	done
