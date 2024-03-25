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

MAKEFLAGS += --no-builtin-rules

export CGO_ENABLED=1

GO := go

GO_FILES := $(shell find . -type f -name '*.go')
PKG_GO_FILES := $(shell find ../../pkg/ -type f -name '*.go')
ALL_GO_FILES := $(GO_FILES) $(PKG_GO_FILES)
GO_TEST_FILES := $(shell find . -type f -name '*_text.go')

ifeq ($(patsubst %$(VERSION),,$(IMAGENAME)),)
else
$(error "$(IMAGENAME) doesn't end with $(VERSION). Please set the correct version.")
endif

build: $(GO_TEST_FILES)
	$(GO) test $(GO_TEST_ARGS) ./... -c

build-pr: build

build-main: build

.PHONY: cleanup-pr
cleanup-pr:

.PHONY: cleanup-main
cleanup-main:

test: $(ALL_GO_FILES)
	$(GO) test $(GO_TEST_ARGS) ./...

all: build test
