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
MIN_COVERAGE=67.1
#SERVICE_DIR=/kp/cli

include ../infrastructure/make/go/include.mk

test: unit-test
build: lint test docker

build-pr: IMAGE_TAG=pr-$(VERSION)
build-pr: build release

build-main: IMAGE_TAG=main-$(VERSION)
build-main: build release

.PHONY: lint
lint:
	$(MAKE) -C $(ROOT_DIR)/pkg gen
	docker run --rm -w /kp/ -v $(shell pwd)/..:/kp/ $(BUILDER_IMAGE) sh -c 'cd cli && GOFLAGS="-buildvcs=false" golangci-lint run --timeout=15m -j4 --tests=false --skip-files=".*\.pb\.go$$" ./...'
