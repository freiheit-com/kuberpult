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


MAKEDIRS := services/cd-service services/rollout-service services/frontend-service charts/kuberpult pkg
ARTIFACT_REGISTRY_URI := europe-west3-docker.pkg.dev/fdc-public-docker-registry/kuberpult

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

plan:
	@infrastructure/scripts/execution-plan/plan-pr.sh

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
	IMAGE_TAG=latest make -C infrastructure/docker/builder build

compose-down:
	docker compose down

kuberpult: compose-down kuberpult-earthly

kuberpult-earthly: compose-down
	earthly +all-services --UID=$(USER_UID)
	docker compose up 

all-services:
	earthly +all-services --tag=$(VERSION)

integration-test:
	earthly -P +integration-test --kuberpult_version=$(IMAGE_TAG_KUBERPULT)

pull-service-image/%:
	docker pull $(DOCKER_REGISTRY_URI)/$*:$(VERSION)
	docker pull $(ARTIFACT_REGISTRY_URI)/$*:$(VERSION)-datadog

tag-service-image/%: pull-service-image/%
	docker tag $(DOCKER_REGISTRY_URI)/$*:$(VERSION) $(DOCKER_REGISTRY_URI)/$*:$(RELEASE_IMAGE_TAG)
	docker tag $(ARTIFACT_REGISTRY_URI)/$*:$(VERSION)-datadog $(ARTIFACT_REGISTRY_URI)/$*:$(RELEASE_IMAGE_TAG)-datadog
	docker tag $(ARTIFACT_REGISTRY_URI)/$*:$(VERSION)-datadog $(DOCKER_REGISTRY_URI)/$*:$(RELEASE_IMAGE_TAG)-datadog

push-service-image/%: tag-service-image/%
	docker push $(DOCKER_REGISTRY_URI)/$*:$(RELEASE_IMAGE_TAG)
	docker push $(ARTIFACT_REGISTRY_URI)/$*:$(RELEASE_IMAGE_TAG)-datadog
	docker push $(DOCKER_REGISTRY_URI)/$*:$(RELEASE_IMAGE_TAG)-datadog

.PHONY: tag-release-images
tag-release-images: $(foreach i,$(SERVICE_IMAGES),push-service-image/$i)
	true

# CLI is only stored in gcp docker registry
pull-cli-image:
	docker pull $(DOCKER_REGISTRY_URI)/$(CLI_IMAGE):$(VERSION)

tag-cli-image: pull-cli-image
	docker tag $(DOCKER_REGISTRY_URI)/$(CLI_IMAGE):$(VERSION) $(DOCKER_REGISTRY_URI)/$(CLI_IMAGE):$(RELEASE_IMAGE_TAG)

push-cli-image: tag-cli-image
	docker push $(DOCKER_REGISTRY_URI)/$(CLI_IMAGE):$(RELEASE_IMAGE_TAG)

.PHONY: tag-cli-image
tag-cli-release-image: push-cli-image
	true

.PHONY: commitlint
commitlint:
	gh pr view $${GITHUB_HEAD_REF} --json title,body --template '{{.title}}{{ "\n\n" }}{{.body}}' > commitlint.msg
	@echo "commit message that will be linted:"
	@cat commitlint.msg
	@echo
	earthly +commitlint
	rm commitlint.msg

.PHONY: pull-trivy check-secrets
pull-trivy:
	docker pull aquasec/trivy@sha256:$$(cat ./.trivy-image.SHA256)

check-secrets:
	docker run aquasec/trivy@sha256:$$(cat ./.trivy-image.SHA256) fs --scanners=secret .

.git/hooks/pre-commit: hooks/pre-commit
	cp $< $@

# kuberpult and kuberpult-earthly should install/update the precommit hook as a sideeffect
kuberpult kuberpult-earthly: .git/hooks/pre-commit
