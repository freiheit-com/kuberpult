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
# NOTE: this will be replaced with the etf-golang makefile
MAKEFLAGS += --no-builtin-rules

VERSION=$(shell cat ../../version)
export VERSION

export CGO_ENABLED=1

IMAGENAME?=ghcr.io/freiheit-com/kuberpult-cd-service:${VERSION}

GO := go

GO_FILES := $(shell find . -type f -name '*.go')

ifeq ($(patsubst %$(VERSION),,$(IMAGENAME)),)
else
$(error "$(IMAGENAME) doesn't end with $(VERSION). Please set the correct version.")
endif


proto:
	make -C../../pkg/api all

bin/:
	mkdir -p bin

build: bin/main

# note repository must be named repository otherwise kuberpult cannot see the repository
run: build
	KUBERPULT_GIT_URL="./repository_remote" KUBERPULT_GIT_BRANCH=master ./bin/main

bin/main: bin/ $(GO_FILES) | proto
	$(GO) build -o bin/main ./cmd/server/

clean:
	rm -rf pkg/api/api.gen.go
	rm -rf bin
	docker rmi $(IMAGENAME) || true

test: $(GO_FILES) | proto
	$(GO) test  ./...

docker: bin/main ../../version
	docker build -t $(IMAGENAME) .


# set IMAGENAME=... to the image where you want to deploy (registry of the project)
# e.g IMAGENAME=ghcr.io/replace_me...cd-service:1.2.3 make release
release: docker
	docker push $(IMAGENAME)

all: test docker

.PHONY: all release test docker clean
