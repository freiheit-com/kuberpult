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

.PHONY: build-base-image push-base-image clean release-tag ct-test test-helm test
include ../../Makefile.variables
MAKEFILE_PATH := $(abspath $(word 1, $(MAKEFILE_LIST)))
MAKEFILE_DIR := $(dir $(MAKEFILE_PATH))
MAKEFLAGS += --no-builtin-rules
CHARTS_REGISTRY:=$(DOCKER_REGISTRY_URI)/charts
CHARTS_BASE_IMAGE_URL:=$(CHARTS_REGISTRY):$(VERSION)
BUILDER_IMAGE:=$(DOCKER_REGISTRY_URI)/infrastructure/docker/builder:$(IMAGE_TAG_KUBERPULT)
TGZ_FILE := kuberpult-$(VERSION).tgz

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
	docker run --rm -w /kp -v "$(MAKEFILE_DIR)$(TGZ_FILE):/kp/$(TGZ_FILE)" -v "$(MAKEFILE_DIR)Chart.yaml:/kp/Chart.yaml" -v "$(MAKEFILE_DIR)values.yaml:/kp/values.yaml" -v "$(MAKEFILE_DIR)templates:/kp/templates" -v "$(MAKEFILE_DIR)tests:/kp/tests" $(BUILDER_IMAGE) sh -c "helm lint $(TGZ_FILE) --set git.url=test --set ingress.domainName=kuberpult.example.com && helm dependency build &&  cd tests &&  go mod init test-helm && go mod tidy &&  go test ./..."
	rm $(MAKEFILE_DIR)tests/go.mod $(MAKEFILE_DIR)tests/go.sum

test: ct-test test-helm

clean:
	rm -rf Chart.yaml
	rm -rf ci/test-values.yaml
	rm -rf kuberpult-*.tgz

release-tag: $(TGZ_FILE)
	echo "Creating release via git tag pipeline"

.PHONY: test-helm
test-helm: $(TGZ_FILE) Chart.yaml

all: Chart.yaml
