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

# NOTE: this will be replaced with the etf-golang makefile
ROOT_DIR=..
MAKEFLAGS += --no-builtin-rules
export CGO_ENABLED=1
GO := go
GO_FILES := $(shell find . -type f -name '*.go')
GO_SERVER_GEN := server-gen.go

include $(ROOT_DIR)/infrastructure/make/go/include.mk

all: lint test
test: unit-test

gen: grpc-api-gen publicapi

grpc-api-gen: api/v1/api.EnvironmentConfig_grpc.pb.go
api/v1/api.EnvironmentConfig_grpc.pb.go api/v1/api.pb.go api/v1/api.pb.gw.go api/v1/api_grpc.pb.go: api/v1/api.proto buf.yaml buf.gen.yaml buf.lock
	if grep -q '^import "google/api/annotations.proto"' $< || grep -q '^import "google/api/http.proto"' $< ; then echo 'Instead of using google.api.http protobuf annotations we write HTTP handlers by hand' ; exit 1 ; fi
	buf generate

publicapi: publicapi/$(GO_SERVER_GEN)
publicapi/$(GO_SERVER_GEN): publicapi/api.yaml
	go tool oapi-codegen -generate "std-http-server" -o publicapi/$(GO_SERVER_GEN) -package publicapi publicapi/api.yaml

release: #no-op
clean:
	rm -f api/v1/*.go
	rm -f publicapi/$(GO_SERVER_GEN)
proto: api/v1/api_grpc.pb.go publicapi

build-pr: all
build-main: all
build-all: all

.PHONY: all clean release test
