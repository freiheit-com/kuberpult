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

.PHONY: build-base-image push-base-image clean release-tag ct-test test-helm test kind-test kind-test-gke
include ../../Makefile.variables
MAKEFILE_PATH := $(abspath $(word 1, $(MAKEFILE_LIST)))
MAKEFILE_DIR := $(dir $(MAKEFILE_PATH))
MAKEFLAGS += --no-builtin-rules
CHARTS_REGISTRY:=$(DOCKER_REGISTRY_URI)/charts
CHARTS_BASE_IMAGE_URL:=$(CHARTS_REGISTRY):$(VERSION)
BUILDER_IMAGE:=$(DOCKER_REGISTRY_URI)/infrastructure/docker/builder:$(IMAGE_TAG_KUBERPULT)
TGZ_FILE := kuberpult-$(VERSION).tgz
GO_TEST_ARGS?=

build-base-image:
	@if ! docker image inspect $(CHARTS_BASE_IMAGE_URL) > /dev/null 2>&1; then \
		docker build . -t $(CHARTS_BASE_IMAGE_URL); \
	else \
		echo "Image '$(CHARTS_BASE_IMAGE_URL)' already exists."; \
	fi

push-base-image:
	docker push $(CHARTS_BASE_IMAGE_URL)

Chart.yaml: build-base-image
	touch Chart.yaml
	docker run --rm -w /kp -e VERSION=$(VERSION) -e CHARTS_VERSION=$(VERSION) -w /kp -v "./Chart.yaml:/kp/Chart.yaml" -v "./Chart.yaml.tpl:/kp/Chart.yaml.tpl" $(CHARTS_BASE_IMAGE_URL) \
		sh -c "envsubst < Chart.yaml.tpl > Chart.yaml"

$(TGZ_FILE): Chart.yaml
	touch ./$(TGZ_FILE)
	docker run --rm -w /kp -v "$(MAKEFILE_DIR)Chart.yaml:/kp/Chart.yaml" -v "$(MAKEFILE_DIR)values.yaml:/kp/values.yaml" -v "$(MAKEFILE_DIR)templates:/kp/templates" -v "$(MAKEFILE_DIR)$(TGZ_FILE):/kp/$(TGZ_FILE)" $(CHARTS_BASE_IMAGE_URL) \
		sh -c "helm dependency update && rm -rf charts && helm dependency update && tar -xzvf charts/*.tgz --directory=charts && helm package ."

ci/test-values.yaml: build-base-image
	touch ci/test-values.yaml
	docker run --rm -w /kp -v "./values.yaml:/kp/values.yaml" -v "./ci:/kp/ci" -v "./ci/test-values.yaml:/kp/ci/test-values.yaml" $(CHARTS_BASE_IMAGE_URL) \
		sh -c "grep -o '^[^#]*' values.yaml > ci/test-values.yaml && yq eval-all -i 'select(fileIndex == 0) * select(fileIndex == 1)' ci/test-values.yaml ci/test-values-override.yaml"

ct-test: Chart.yaml
ifeq ($(CI),true)
	@echo "running on CI no need to test this again! Check chart testing action."
else
	docker run --rm -w /kp -v "$(MAKEFILE_DIR)ci:/kp/ci" -v "$(MAKEFILE_DIR)Chart.yaml:/kp/Chart.yaml" -v "$(MAKEFILE_DIR)values.yaml:/kp/values.yaml" -v "$(MAKEFILE_DIR)templates:/kp/templates" $(CHARTS_BASE_IMAGE_URL) \
		sh -c "ct lint --debug --chart-yaml-schema=ci/chart_schema.yaml --lint-conf=ci/lintconf.yaml --chart-dirs=. --charts=. --remote=git@github.com:freiheit-com/kuberpult.git"
endif

test-helm: $(TGZ_FILE) Chart.yaml
	docker run --rm -w /kp -v "$(MAKEFILE_DIR)$(TGZ_FILE):/kp/$(TGZ_FILE)" -v "$(MAKEFILE_DIR)Chart.yaml:/kp/Chart.yaml" -v "$(MAKEFILE_DIR)values.yaml:/kp/values.yaml" -v "$(MAKEFILE_DIR)templates:/kp/templates" -v "$(MAKEFILE_DIR)tests:/kp/tests" $(BUILDER_IMAGE) sh -c "helm lint $(TGZ_FILE) --set git.url=test --set ingress.domainName=kuberpult.example.com && helm dependency build && cd tests && go mod tidy && go test ./..."

test: ct-test test-helm

kind-test:
	@if [ -n "$$(git status --porcelain)" ]; then \
		echo "WARNING: working tree has uncommitted changes. Run charts/kuberpult/run-kind.sh instead (it rebuilds images from your local source)."; \
	fi
	@bash -euo pipefail -c \
		'HASH=$$(git rev-parse HEAD); \
		mkdir -p "$(MAKEFILE_DIR)kind-test-logs/$$HASH"; \
		LOG="$(MAKEFILE_DIR)kind-test-logs/$$HASH/test-$$(date +%Y%m%d-%H%M%S).log"; \
		echo "kind-test: logging to $$LOG"; \
		printf "kuberpult commit: $$HASH\n" | tee "$$LOG"; \
		cd ../../ && go test -count=1 -v $(GO_TEST_ARGS) ./tests/kind-brackets/ -timeout 20m 2>&1 | tee -a "$$LOG"'

kind-test-gke:
	@if [ -z "$(KUBERPULT_TEST_VERSION)" ]; then \
		echo "ERROR: KUBERPULT_TEST_VERSION is required for GKE mode (e.g. v13.52.0)"; \
		exit 1; \
	fi
	@bash -euo pipefail -c \
		'mkdir -p "$(MAKEFILE_DIR)kind-test-logs/$(KUBERPULT_TEST_VERSION)"; \
		LOG="$(MAKEFILE_DIR)kind-test-logs/$(KUBERPULT_TEST_VERSION)/test-$$(date +%Y%m%d-%H%M%S).log"; \
		echo "kind-test-gke: logging to $$LOG"; \
		printf "kuberpult version: $(KUBERPULT_TEST_VERSION)\n" | tee "$$LOG"; \
		cd ../../ && KUBERPULT_TEST_MODE=gke KUBERPULT_TEST_VERSION="$(KUBERPULT_TEST_VERSION)" go test -count=1 -v $(GO_TEST_ARGS) ./tests/kind-brackets/ -timeout 20m 2>&1 | tee -a "$$LOG"'

clean:
	rm -rf Chart.yaml
	rm -rf ci/test-values.yaml
	rm -rf kuberpult-*.tgz

release-tag: $(TGZ_FILE)
	echo "Creating release via git tag pipeline"

.PHONY: test-helm
test-helm: $(TGZ_FILE) Chart.yaml

all: Chart.yaml

.PHONY: lint lint-fix

lint:

lint-fix:

.PHONY: build-all-docker docker-cd-service docker-manifest-repo-export-service docker-reposerver-service docker-rollout-service docker-frontend-service

build-all-docker: docker-cd-service docker-manifest-repo-export-service docker-reposerver-service docker-rollout-service docker-frontend-service

docker-cd-service:
	$(MAKE) -C ../../services/cd-service docker IMAGE_TAG=$(IMAGE_TAG)

docker-manifest-repo-export-service:
	$(MAKE) -C ../../services/manifest-repo-export-service docker IMAGE_TAG=$(IMAGE_TAG)

docker-reposerver-service:
	$(MAKE) -C ../../services/reposerver-service docker IMAGE_TAG=$(IMAGE_TAG)

docker-rollout-service:
	$(MAKE) -C ../../services/rollout-service docker IMAGE_TAG=$(IMAGE_TAG)

docker-frontend-service:
	$(MAKE) -C ../../services/frontend-service docker gen-api IMAGE_TAG=$(IMAGE_TAG)
