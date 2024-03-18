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
IMAGE_REGISTRY?=ghcr.io/freiheit-com/kuberpult
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
EARTHLY := earthly

image-name:
	@echo "$(IMAGENAME)"

version:
	@echo "$(VERSION)"

proto:
	make -C../../pkg/api all

bin/:
	mkdir -p bin

build: bin/main

build-pr:
	echo "build on pull request"
	$(EARTHLY) --push +build-pr --registry=$(IMAGE_REGISTRY) --tag=$(VERSION) --mirror=$(ARTIFACT_REGISTRY_MIRROR)

build-main:
	echo "build on main"
	$(EARTHLY) --push +build-main --registry=$(IMAGE_REGISTRY) --tag=$(VERSION) --mirror=$(ARTIFACT_REGISTRY_MIRROR)

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
	$(EARTHLY) +unit-test "--GO_TEST_ARGS=$(GO_TEST_ARGS)"

docker:
	$(EARTHLY) +docker --registry=$(IMAGE_REGISTRY) --tag=$(VERSION)

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
