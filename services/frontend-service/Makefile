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

VERSION=$(shell cat ../../version)
export VERSION

export CGO_ENABLED=0

GO := go

GO_FILES := $(shell find . -type f -name '*.go')

DOCKER_REPOSITORY := eu.gcr.io/freiheit-core/fdc-continuous-delivery

GARBAGE := *.sentinel

proto:
	$(MAKE) -C../../pkg/api all

bin/main: $(GO_FILES) | proto
	mkdir -p bin
	$(GO) build -o bin/main ./cmd/server/
GARBAGE += bin/main

run: bin/main
	KUBERPULT_CDSERVER=localhost:8443 bin/main

clean:
	git clean -Xfd

test: $(GO_FILES) proto
	$(GO) test ./...

docker: bin/main ../../version | dist
	docker build -t ${DOCKER_REPOSITORY}/frontend-service:${VERSION} .

release: docker
	docker push ${DOCKER_REPOSITORY}/frontend-service:${VERSION}

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
