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

# NOTE: this will be replaced with the etf-golang makefile

include ../../Makefile.variables

MAKEFLAGS += --no-builtin-rules

export CGO_ENABLED=1

IMAGENAME?=$(IMAGE_REGISTRY)/kuberpult-cd-service:$(VERSION)
ARTIFACT_REGISTRY_IMAGE_NAME=europe-west3-docker.pkg.dev/fdc-public-docker-registry/kuberpult/kuberpult-cd-service:${VERSION}

export KUBERPULT_DEX_MOCK=false
export KUBERPULT_DEX_ENABLED=false

ifeq ($(WITH_DOCKER),)
COMPILE_WITH_DOCKER := false
else
COMPILE_WITH_DOCKER := true
endif

GO := go

GO_FILES := $(shell find . -type f -name '*.go')
PKG_GO_FILES := $(shell find ../../pkg/ -type f -name '*.go')
ALL_GO_FILES := $(GO_FILES) $(PKG_GO_FILES)

ifeq ($(patsubst %$(VERSION),,$(IMAGENAME)),)
else
$(error "$(IMAGENAME) doesn't end with $(VERSION). Please set the correct version.")
endif

SERVICE_NAME := $(shell basename $$PWD)
EARTHLY := earthly --remote-cache=ghcr.io/freiheit-com/kuberpult/kuberpult-$(SERVICE_NAME):cache

image-name:
	@echo "$(IMAGENAME)"

version:
	@echo "$(VERSION)"

proto:
	make -C../../pkg/api all

bin/:
	mkdir -p bin

build: bin/main

build-pr: release-pr
	echo "build on pull request"

build-main: test release-main
	echo "build on main"

.PHONY: cleanup-pr
cleanup-pr:
	echo "Nothing to clean"

.PHONY: cleanup-main
cleanup-main:
	echo "Nothing to clean"

bin/main: bin/ $(ALL_GO_FILES) | proto
	@echo "building without docker, just plain go"
	$(GO) build -o bin/main ./cmd/server/

clean:
	rm -rf pkg/api/api.gen.go
	rm -rf bin
	docker rmi $(IMAGENAME) || true

.PHONY: test-dependancies
test-dependancies:
	make -C ../../pkg test
	make -C ../../pkg/api test

test:
	$(EARTHLY) +unit-test

docker:
	$(EARTHLY) +docker --registry=$(IMAGE_REGISTRY) --tag=$(VERSION)

# set IMAGENAME=... to the image where you want to deploy (registry of the project)
# e.g IMAGENAME=ghcr.io/replace_me...cd-service:1.2.3 make release
release-pr:
	$(EARTHLY) --push +release --registry=$(IMAGE_REGISTRY) --tag=$(VERSION)
ifeq ($(ARTIFACT_REGISTRY_MIRROR), true)
	$(EARTHLY) --push +release --registry=europe-west3-docker.pkg.dev/fdc-public-docker-registry/kuberpult --tag=$(VERSION)
endif

release-main:
	$(EARTHLY) --push --max-remote-cache +release --registry=$(IMAGE_REGISTRY) --tag=$(VERSION)
ifeq ($(ARTIFACT_REGISTRY_MIRROR), true)
	$(EARTHLY) --push --max-remote-cache +release --registry=europe-west3-docker.pkg.dev/fdc-public-docker-registry/kuberpult --tag=$(VERSION)
endif

all: test docker

.PHONY: publish
publish: release

.PHONY: all release test docker clean

.PHONY: get-builder-image
get-builder-image:
	@echo "$(KUBERPULT_BUILDER)"

kind-load: docker
	kind load docker-image "$(IMAGENAME)"

patch-kind: kind-load
	kubectl set image deployment/kuberpult-cd-service service=$(IMAGENAME)
