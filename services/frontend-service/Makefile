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


include ../../Makefile.variables

MAKEFLAGS += --no-builtin-rules

export CGO_ENABLED=0

IMAGE_REGISTRY?=ghcr.io/freiheit-com/kuberpult
GO := go

GO_FILES := $(shell find . -type f -name '*.go')
PKG_GO_FILES := $(shell find ../../pkg/ -type f -name '*.go')
ALL_GO_FILES := $(GO_FILES) $(PKG_GO_FILES)


IMAGENAME?=$(IMAGE_REGISTRY)/kuberpult-frontend-service:${VERSION}
ARTIFACT_REGISTRY_IMAGE_NAME=europe-west3-docker.pkg.dev/fdc-public-docker-registry/kuberpult/kuberpult-frontend-service:${VERSION}
GARBAGE := *.sentinel

SERVICE_NAME := $(shell basename $$PWD)
EARTHLY := earthly
image-name:
	@echo "$(IMAGENAME)"

proto:
	$(MAKE) -C../../pkg/api all

bin/main: $(ALL_GO_FILES) | proto
	mkdir -p bin
	$(GO) build -o bin/main ./cmd/server/
GARBAGE += bin/main

export KUBERPULT_CDSERVER?=localhost:8443
export KUBERPULT_ALLOWED_ORIGINS?=localhost:*
export KUBERPULT_ARGOCD_BASE_URL=https://cd.dev.freiheit.systems/
export KUBERPULT_SOURCE_REPO_URL=https://github.com/freiheit-com/kuberpult/commit/{commit}
export KUBERPULT_MANIFEST_REPO_URL=https://github.com/freiheit-com/fdc-standard-setup-dev-env-manifest/tree/{branch}/{dir}
export KUBERPULT_GIT_BRANCH=main
run: bin/main
	./bin/main

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

.PHONY: publish
publish: release

start: buf-generate
	pnpm i && pnpm start

clean:
	git clean -Xfd

.PHONY: test-dependencies
test-dependencies:
	make -C ../../pkg test
	make -C ../../pkg/api test

test:
	$(EARTHLY) +unit-test "--GO_TEST_ARGS=$(GO_TEST_ARGS)"
	$(EARTHLY) +unit-test-ui

test-go: $(ALL_GO_FILES) | proto
	$(GO) test $(GO_TEST_ARGS) ./...

test-ts: src/api/api.ts
	pnpm eslint-check
	# The extra '--' was added because of pnpm, Usage: pnpm test [-- <args>...]
	pnpm test -- --watchAll=false

scss-check: deps
	pnpm scss-check

docker:
	$(EARTHLY) +docker --registry=$(IMAGE_REGISTRY) --tag=$(VERSION)

all: test docker

.PHONY: all release test docker clean

## Node part
NODE_BIN_DIR=$(shell pnpm bin)

deps: .deps.sentinel

.deps.sentinel: package.json
	pnpm i && touch .deps.sentinel

src/api/api.ts: ../../pkg/api/v1/api.proto | deps
buf-generate: src/api/api.ts
	mkdir -p src/api
	buf generate --path api --timeout 5m

dist: .dist.sentinel

.dist.sentinel: buf-generate $(shell find public src -type f)
	rm -rf build/*
	pnpm build && touch .dist.sentinel


.DEFAULT_GOAL := all

.PHONY: deps

.PHONY: get-builder-image
get-builder-image:
	@echo "$(KUBERPULT_BUILDER)"
