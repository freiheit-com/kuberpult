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
MAKEFLAGS += --no-builtin-rules
GO := go
GO_FILES := $(shell find . -type f -name '*.go')
GO_SERVER_GEN := server-gen.go
MIN_COVERAGE?=47
MAIN_PATH=

SERVICE_DIR:=/kp/pkg
include $(ROOT_DIR)/infrastructure/make/go/include.mk

PKG_WITHOUT_DOCKER?=0

all: gen lint test
test: gen unit-test

.PHONY: lint
lint: gen
ifeq ($(PKG_WITHOUT_DOCKER),1)
	@echo "Running without docker"
	(cd .. && GOFLAGS="-buildvcs=false" golangci-lint run --timeout=25m -j4 --tests=false --skip-files=".*\.pb\.go$$" ./pkg/... || $(SKIP_LINT_ERRORS) && exhaustruct -test=false $$(go list ./pkg/... | grep -v "github.com/freiheit-com/kuberpult/pkg/api" | grep -v "github.com/freiheit-com/kuberpult/pkg/publicapi"))
else
	@echo "Running in docker"
	docker run --rm -w /kp -v "..:/kp"    $(DEPS_IMAGE) sh -c 'export GOFLAGS="-buildvcs=false"; golangci-lint run --timeout=25m -j4 --tests=false --skip-files=".*\.pb\.go$$" ./pkg/... || $(SKIP_LINT_ERRORS) && exhaustruct -test=false $$(GOFLAGS="-buildvcs=false" go list ./... | grep -v "github.com/freiheit-com/kuberpult/pkg/api" | grep -v "github.com/freiheit-com/kuberpult/pkg/publicapi")'
endif

gen: grpc-api-gen publicapi

grpc-api-gen: api/v1/api.EnvironmentConfig_grpc.pb.go
api/v1/api.EnvironmentConfig_grpc.pb.go api/v1/api.pb.go api/v1/api.pb.gw.go api/v1/api_grpc.pb.go: api/v1/api.proto buf.yaml buf.gen.yaml buf.lock deps
	if grep -q '^import "google/api/annotations.proto"' $< || grep -q '^import "google/api/http.proto"' $< ; then echo 'Instead of using google.api.http protobuf annotations we write HTTP handlers by hand' ; exit 1 ; fi
ifeq ($(PKG_WITHOUT_DOCKER),1)
	@echo "Running without docker"
	buf generate
else
	@echo "Running in docker"
	docker run -w /kp/pkg --rm -v "./api/v1:/kp/pkg/api/v1" $(DEPS_IMAGE) sh -c "buf generate"
endif

publicapi: publicapi/$(GO_SERVER_GEN)
publicapi/$(GO_SERVER_GEN): publicapi/api.yaml
ifeq ($(PKG_WITHOUT_DOCKER),1)
	@echo "Running without docker"
	buf generate && go tool oapi-codegen -generate "std-http-server" -o publicapi/$(GO_SERVER_GEN) -package publicapi publicapi/api.yaml
else
	@echo "Running in docker"
	docker run -w /kp/pkg --rm -v "./publicapi:/kp/pkg/publicapi" $(DEPS_IMAGE) sh -c 'buf generate && go tool oapi-codegen -generate "std-http-server" -o publicapi/$(GO_SERVER_GEN) -package publicapi publicapi/api.yaml'
endif

release: #no-op
clean:
	rm -f api/v1/*.go
	rm -f publicapi/$(GO_SERVER_GEN)
proto: api/v1/api_grpc.pb.go publicapi

build-pr: all
build-main: all

.PHONY: all clean release test
