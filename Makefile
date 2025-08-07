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
SHELL := sh

include ./Makefile.variables
MAKEFLAGS += --no-builtin-rules

SCRIPTS_BASE:=infrastructure/scripts/make


MAKEDIRS := services/cd-service services/rollout-service services/frontend-service services/reposerver-service charts/kuberpult pkg
INTEGRATION_TEST_IMAGE ?=$(DOCKER_REGISTRY_URI)/integration-test:$(IMAGE_TAG)
ARTIFACT_REGISTRY_URI := europe-west3-docker.pkg.dev/fdc-public-docker-registry/kuberpult
INTEGRATION_TEST_CONFIG_DIR := tests/integration-tests/cluster-setup/config
INTEGRATION_TEST_CONFIG_FILE := $(INTEGRATION_TEST_CONFIG_DIR)/kubeconfig.yaml
COMMIT_MSG_FILE := commitlint.msg

export USER_UID := $(shell id -u)
.install:
	touch .install

$(addsuffix /release,$(MAKEDIRS)):
	make -C $(dir $@) release

release: $(addsuffix /release,$(MAKEDIRS))
	git tag $(VERSION)

$(addsuffix /clean,$(MAKEDIRS)):
	make -C $(dir $@) clean

clean: $(addsuffix /clean,$(MAKEDIRS))

$(addsuffix /test,$(MAKEDIRS)):
	make -C $(dir $@) test

test: $(addsuffix /test,$(MAKEDIRS))

$(addsuffix /all,$(MAKEDIRS)):
	make -C $(dir $@) all

all: $(addsuffix /all,$(MAKEDIRS))

init:

.PHONY: release  $(addsuffix /release,$(MAKEDIRS)) all $(addsuffix /all,$(MAKEDIRS)) clean $(addsuffix /clean,$(MAKEDIRS))

.PHONY: version
version:
	@echo $(VERSION)

.PHONY: cleanup-pr
cleanup-pr:
	@echo "Nothing to do"

.PHONY: cleanup-main
cleanup-main:
	@echo "Nothing to do"

.PHONY: builder
builder:
	IMAGE_TAG=local make -C infrastructure/docker/builder build

compose-down:
	docker compose down

prepare-compose: builder
	IMAGE_TAG=local make -C services/cd-service docker
	IMAGE_TAG=local make -C services/manifest-repo-export-service docker
	IMAGE_TAG=local make -C services/frontend-service docker

kuberpult: prepare-compose compose-down
	docker compose -f docker-compose.yml -f docker-compose.persist.yml up

reset-db: compose-down
	# This deletes the volume of the default db location:
	docker volume rm kuberpult_pgdata

kuberpult-freshdb: prepare-compose compose-down
	docker compose up

# Run this before starting the unit tests in your IDE:
unit-test-db:
	docker compose -f docker-compose-unittest.yml up

DATA_DIR:=$(shell echo ~/.k3s-cache)

###
# INTEGRATION_TEST_CONFIG_DIR := tests/integration-tests/cluster-setup/config
# INTEGRATION_TEST_CONFIG_FILE := $(INTEGRATION_TEST_CONFIG_DIR)/kubeconfig.yaml

integration-test:
	#IMAGE_TAG=$(IMAGE_TAG_KUBERPULT) make -C ./pkg gen
	mkdir -p $(INTEGRATION_TEST_CONFIG_DIR)
	rm -f $(INTEGRATION_TEST_CONFIG_FILE)
	sudo rm -rf ${DATA_DIR}/*
	sudo chown -R su:su ${DATA_DIR}
	sudo chmod -R 700 ${DATA_DIR}
	K3S_TOKEN="Random" docker compose -f tests/integration-tests/cluster-setup/docker-compose-k3s-B.yml down
	docker volume rm -f cluster-setup_k3s-server
	# cleanup done


	docker compose -f tests/integration-tests/cluster-setup/docker-compose-k3s-B.yml up -d k3s-server --remove-orphans
	K3S_TOKEN=$$(docker exec k3s-server cat /var/lib/rancher/k3s/server/node-token) docker compose -f tests/integration-tests/cluster-setup/docker-compose-k3s-B.yml up -d k3s-agent
	#echo TOKEN: $$(docker exec k3s-server cat /var/lib/rancher/k3s/server/node-token)
	#echo "TOKEN: ${K3S_TOKEN}"
	#docker compose -f tests/integration-tests/cluster-setup/docker-compose-k3s-B.yml up -d k3s-agent --remove-orphans

#	while [ ! -s "$(INTEGRATION_TEST_CONFIG_FILE)" ]; do \
#		sleep 1; \
#	done;
#	sed -i -e 's|6443|8443|g' $(INTEGRATION_TEST_CONFIG_FILE)
	docker build -f tests/integration-tests/Dockerfile . -t $(INTEGRATION_TEST_IMAGE) --build-arg kuberpult_version=$(IMAGE_TAG_KUBERPULT) --build-arg charts_version=$(VERSION)
	docker run  --network=host -v "./$(INTEGRATION_TEST_CONFIG_FILE):/kp/kubeconfig.yaml" --rm $(INTEGRATION_TEST_IMAGE)
#	rm -f $(INTEGRATION_TEST_CONFIG_FILE)

pull-service-image/%:
	docker pull $(DOCKER_REGISTRY_URI)/$*:main-$(VERSION)

tag-service-image/%: pull-service-image/%
	docker tag $(DOCKER_REGISTRY_URI)/$*:main-$(VERSION) $(DOCKER_REGISTRY_URI)/$*:$(RELEASE_IMAGE_TAG)

push-service-image/%: tag-service-image/%
	docker push $(DOCKER_REGISTRY_URI)/$*:$(RELEASE_IMAGE_TAG)

.PHONY: tag-release-images
tag-release-images: $(foreach i,$(SERVICE_IMAGES),push-service-image/$i)
	true

# CLI is only stored in gcp docker registry
pull-cli-image:
	docker pull $(DOCKER_REGISTRY_URI)/$(CLI_IMAGE):main-$(VERSION)

tag-cli-image: pull-cli-image
	docker tag $(DOCKER_REGISTRY_URI)/$(CLI_IMAGE):main-$(VERSION) $(DOCKER_REGISTRY_URI)/$(CLI_IMAGE):$(RELEASE_IMAGE_TAG)

push-cli-image: tag-cli-image
	docker push $(DOCKER_REGISTRY_URI)/$(CLI_IMAGE):$(RELEASE_IMAGE_TAG)

.PHONY: tag-cli-image
tag-cli-release-image: push-cli-image
	true

.PHONY: commitlint
commitlint: $(COMMIT_MSG_FILE)
	docker run -w /commitlint -v "./commitlint.config.js:/commitlint/commitlint.config.js" -v "./$(COMMIT_MSG_FILE):/commitlint/$(COMMIT_MSG_FILE)" node:18-bookworm sh -c "npm install --save-dev @commitlint/cli@18.4.3 && cat ./$(COMMIT_MSG_FILE) | npx commitlint --config commitlint.config.js"
	rm $(COMMIT_MSG_FILE)

$(COMMIT_MSG_FILE):
	git log -1 --pretty=%B > $(COMMIT_MSG_FILE)

.PHONY: pull-trivy check-secrets
pull-trivy:
	docker pull aquasec/trivy@sha256:$$(cat ./.trivy-image.SHA256)

check-secrets:
	docker run aquasec/trivy@sha256:$$(cat ./.trivy-image.SHA256) fs --scanners=secret .

.git/hooks/pre-commit: hooks/pre-commit
	cp $< $@

.git/hooks/commit-msg: hooks/commit-msg
	cp $< $@

kuberpult: .git/hooks/pre-commit .git/hooks/commit-msg
