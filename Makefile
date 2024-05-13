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

# Copyright 2023 freiheit.com
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

kuberpult: builder
	make -C services/frontend-service src/api/api.ts
	make -C pkg/ all
	docker compose up --build

kuberpult-earthly:
	earthly +all-services --UID=$(USER_UID) --target docker
	docker compose -f docker-compose-earthly.yml up 

cache:
	earthly --remote-cache=ghcr.io/freiheit-com/kuberpult/kuberpult-frontend-service:cache --push +frontend-service --target release --UID=$(USER_UID)
	earthly --remote-cache=ghcr.io/freiheit-com/kuberpult/kuberpult-cd-service:cache --push +cd-service --UID=$(USER_UID) --target release
	earthly --remote-cache=ghcr.io/freiheit-com/kuberpult/kuberpult-rollout-service:cache --push +rollout-service --UID=$(USER_UID) --target release

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

.PHONY: commitlint
commitlint:
	gh pr view $${GITHUB_HEAD_REF} --json title,body --template '{{.title}}{{ "\n\n" }}{{.body}}' > commitlint.msg
	@echo "commit message that will be linted:"
	@cat commitlint.msg
	@echo
	earthly +commitlint
	rm commitlint.msg
