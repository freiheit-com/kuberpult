#This file is part of kuberpult.

#Kuberpult is free software: you can redistribute it and/or modify
#it under the terms of the GNU General Public License as published by
#the Free Software Foundation, either version 3 of the License, or
#(at your option) any later version.

#Kuberpult is distributed in the hope that it will be useful,
#but WITHOUT ANY WARRANTY; without even the implied warranty of
#MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
#GNU General Public License for more details.

#You should have received a copy of the GNU General Public License
#along with kuberpult.  If not, see <http://www.gnu.org/licenses/>.

#Copyright 2021 freiheit.com
MAKEFLAGS += --no-builtin-rules
CUR_DIR := $(shell cd -- "$(dirname "$0")" >/dev/null 2>&1 ; pwd -P) 
VERSION=$(shell cat ../../version)
export VERSION

export CGO_ENABLED=0

GO := go

GO_FILES := $(shell find . -type f -name '*.go')


IMAGENAME?=ghcr.io/freiheit-com/kuberpult-frontend-service:${VERSION}

GARBAGE := *.sentinel

proto:
	$(MAKE) -C../../pkg/api all

bin/main: $(GO_FILES) | proto
	mkdir -p bin
	$(GO) build -o bin/main ./cmd/server/
GARBAGE += bin/main

export KUBERPULT_CDSERVER?=localhost:8443
run: bin/main
	./bin/main

build: bin/main

build-pr: docker
	echo "build on pull request"

build-main: docker
	echo "build on main"

publish: release

start: src/api/api.ts
	yarn && yarn start

clean:
	git clean -Xfd

test: test-go test-ts

test-go: $(GO_FILES) proto
	$(GO) test ./...

test-ts: src/api/api.ts
	yarn eslint
	yarn test --watchAll=false

docker: bin/main ../../version | dist
	docker build -t $(IMAGENAME) .

release: docker
	$(CURDIR)/../../infrastructure/scripts/execution-plan/push.sh $(IMAGENAME)

all: test docker

.PHONY: all release test docker clean

## Node part
NODE_BIN_DIR=$(shell yarn bin)

deps: .deps.sentinel

.deps.sentinel: package.json
	yarn install && touch .deps.sentinel

src/api/api.ts: ../../pkg/api/api.proto | deps
	mkdir -p src/api
	buf generate --path api

dist: .dist.sentinel

.dist.sentinel: src/api/api.ts $(shell find public src -type f)
	rm -rf build/*
	yarn build && touch .dist.sentinel


.DEFAULT_GOAL := all

.PHONY: deps
