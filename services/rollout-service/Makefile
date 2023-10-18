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


include ../../Makefile.variables

MAKEFLAGS += --no-builtin-rules

export CGO_ENABLED=0

GO := go

GO_FILES := $(shell find . -type f -name '*.go')
PKG_GO_FILES := $(shell find ../../pkg/ -type f -name '*.go')
ALL_GO_FILES := $(GO_FILES) $(PKG_GO_FILES)

GIT_VERSION := $(shell git rev-parse HEAD)

IMAGENAME?=$(IMAGE_REGISTRY)/kuberpult-rollout-service:${VERSION}
ARTIFACT_REGISTRY_IMAGE_NAME=europe-west3-docker.pkg.dev/fdc-public-docker-registry/kuberpult/kuberpult-rollout-service:${VERSION}

GARBAGE := *.sentinel

proto:
	$(MAKE) -C../../pkg/api all

bin/main: $(ALL_GO_FILES) | proto
	mkdir -p bin
	$(GO) build -o bin/main ./cmd/server/
GARBAGE += bin/main

export KUBERPULT_CDSERVER?=localhost:8443
run: bin/main
	./bin/main

build: bin/main

build-pr: test docker publish
	echo "build on pull request"

build-main: test docker publish
	echo "build on main"

.PHONY: cleanup-pr
cleanup-pr:
	echo "Nothing to clean"

.PHONY: cleanup-main
cleanup-main:
	echo "Nothing to clean"

.PHONY: publish
publish: release

clean:
	git clean -Xfd

test: test-go

test-go: $(GO_FILES) proto
	$(GO) test ./...

docker:
	echo GIT_VERSION: $(GIT_VERSION)
	docker build -t $(IMAGENAME) ../.. -f Dockerfile --build-arg="GIT_VERSION=$(GIT_VERSION)" --build-arg="DIR=/services/rollout-service"

release: docker
	docker push $(IMAGENAME)
ifeq ($(ARTIFACT_REGISTRY_MIRROR), true)
	docker tag $(IMAGENAME) $(ARTIFACT_REGISTRY_IMAGE_NAME)
	docker push $(ARTIFACT_REGISTRY_IMAGE_NAME)
endif
all: test docker

.PHONY: all release test docker clean

.DEFAULT_GOAL := all

.PHONY: deps

.PHONY: get-builder-image
get-builder-image:
	@echo "$(KUBERPULT_BUILDER)"

kind-load: docker
	kind load docker-image "$(IMAGENAME)"

patch-kind: kind-load
	kubectl set image deployment/kuberpult-rollout-service service=$(IMAGENAME)
