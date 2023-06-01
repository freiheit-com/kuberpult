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

ifeq ($(WITH_DOCKER),)
COMPILE_WITH_DOCKER := false
else
COMPILE_WITH_DOCKER := true
endif

GO := go

GO_FILES := $(shell find . -type f -name '*.go')

ifeq ($(patsubst %$(VERSION),,$(IMAGENAME)),)
else
$(error "$(IMAGENAME) doesn't end with $(VERSION). Please set the correct version.")
endif

image-name:
	@echo "$(IMAGENAME)"

proto:
	make -C../../pkg/api all

bin/:
	mkdir -p bin

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

ifeq ($(COMPILE_WITH_DOCKER),true)

# note repository must be named repository otherwise kuberpult cannot see the repository
run: docker
	echo $(shell pwd)
	docker run --rm -e KUBERPULT_GIT_URL="/repository_remote" -e KUBERPULT_GIT_BRANCH=master -p 8080:8080 -p 8443:8443 -v $(shell pwd)/repository_remote:/repository_remote $(IMAGENAME)
else
run: build
	KUBERPULT_GIT_URL="./repository_remote" KUBERPULT_GIT_BRANCH=master ./bin/main
endif

bin/main: bin/ $(GO_FILES) | proto
ifeq ($(COMPILE_WITH_DOCKER),true)
# for "libgit reasons", we need the option to build in docker.
	@echo "building within docker (dmake)"
	../../dmake make -C services/cd-service bin/main
else
	@echo "building without docker, just plain go"
	$(GO) build -o bin/main ./cmd/server/
endif


clean:
	rm -rf pkg/api/api.gen.go
	rm -rf bin
	docker rmi $(IMAGENAME) || true

.PHONY: test-dependancies
test-dependancies:
	echo SKIPPING TEST TODO
#	make -C ../../pkg test
#	make -C ../../pkg/api test

test: $(GO_FILES) test-dependancies | proto
	echo SKIPPING TEST TODO
#	$(GO) test  ./...

docker: bin/main
	docker build -t $(IMAGENAME) .


# set IMAGENAME=... to the image where you want to deploy (registry of the project)
# e.g IMAGENAME=ghcr.io/replace_me...cd-service:1.2.3 make release
release: docker
	docker push $(IMAGENAME)

all: test docker

.PHONY: publish
publish: release

.PHONY: all release test docker clean

.PHONY: get-builder-image
get-builder-image:
	@echo "$(KUBERPULT_BUILDER)"
