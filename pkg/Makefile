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
include ../Makefile.variables
MAKEFLAGS += --no-builtin-rules

export CGO_ENABLED=1

GO := go

GO_FILES := $(shell find . -type f -name '*.go')

all:
	earthly +build-all
lint:
	earthly +lint
test:
	earthly +test "--GO_TEST_ARGS=$(GO_TEST_ARGS)"

api/v1/api.EnvironmentConfig_grpc.pb.go api/v1/api.pb.go api/v1/api.pb.gw.go api/v1/api_grpc.pb.go: api/v1/api.proto buf.yaml buf.gen.yaml buf.lock
	if grep -q '^import "google/api/annotations.proto"' $< || grep -q '^import "google/api/http.proto"' $< ; then echo 'Instead of using google.api.http protobuf annotations we write HTTP handlers by hand' ; exit 1 ; fi
	buf generate

release: #no-op
clean:
	rm -f api/v1/*.go
proto: api/v1/api_grpc.pb.go

build-pr: all
build-main: all

.PHONY: all clean release test
