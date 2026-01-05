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

ROOT_DIR=..
CGO_ENABLED=0
MAIN_PATH=cmd/kuberpult-client
SERVICE=client
MIN_COVERAGE=66.6
MAX_DOCKER_SIZE_MB=10.7
CONTEXT?=../

include ../infrastructure/make/go/include.mk

test: unit-test
build: lint test docker

build-pr-internal: build release
build-pr:
	IMAGE_TAG=pr-$(VERSION) BUILDER_IMAGE=$(DOCKER_REGISTRY_URI)/infrastructure/docker/builder:pr-$(VERSION) $(MAKE) build-pr-internal

build-main-internal: build release
build-main:
	IMAGE_TAG=main-$(VERSION) $(MAKE) build-main-internal

.PHONY: lint
lint:
	IMAGE_TAG=$(IMAGE_TAG_KUBERPULT) $(MAKE) -C $(ROOT_DIR)/pkg gen
	docker run --rm -w /kp/ -v $(shell pwd)/..:/kp/ $(BUILDER_IMAGE) sh -c 'cd cli && GOFLAGS="-buildvcs=false" golangci-lint run --timeout=15m -j4 --tests=false ./...'
