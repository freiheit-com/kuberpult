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

SCRIPTS_BASE:=infrastructure/scripts/make
CODE_REVIEWER_LOCATION?=$(HOME)/bin/codereviewr

VERSION=$(shell cat version)
export VERSION

MAKEDIRS := services/cd-service services/frontend-service chart pkg/api pkg

.install: go.tools.mod go.tools.sum
	 go install -modfile=go.tools.mod github.com/golang/protobuf/protoc-gen-go google.golang.org/grpc/cmd/protoc-gen-go-grpc && \
              go get github.com/ghodss/yaml@v1.0.0 && \
              go install github.com/libgit2/git2go/v33 && \
              go install \
                github.com/grpc-ecosystem/grpc-gateway/v2/protoc-gen-grpc-gateway \
                github.com/grpc-ecosystem/grpc-gateway/v2/protoc-gen-openapiv2 \
                google.golang.org/protobuf/cmd/protoc-gen-go \
                github.com/grpc-ecosystem/grpc-gateway/v2/internal/descriptor
	touch .install

$(addsuffix /release,$(MAKEDIRS)):
	make -C $(dir $@) release

release: $(addsuffix /release,$(MAKEDIRS)) version
	git tag $(VERSION)

$(addsuffix /clean,$(MAKEDIRS)):
	make -C $(dir $@) clean

clean: $(addsuffix /clean,$(MAKEDIRS))

$(addsuffix /test,$(MAKEDIRS)):
	make -C $(dir $@) test

test: $(addsuffix /test,$(MAKEDIRS))

$(addsuffix /all,$(MAKEDIRS)):
	make -C $(dir $@) all

all: $(addsuffix /all,$(MAKEDIRS))


$(CODE_REVIEWER_LOCATION):
ifeq ($(CI),true)
	@wget -O /tmp/codereviewr https://storage.googleapis.com/codereviewr_a7ed108e-470d-4be0-b5bc-001e4d64f0a2/latest/codereviewr
	install -m 755 /tmp/codereviewr $@
else
	@wget -O /tmp/codereviewr https://storage.googleapis.com/codereviewr_a7ed108e-470d-4be0-b5bc-001e4d64f0a2/latest/codereviewr
	install -m 755 /tmp/codereviewr $@
endif

analyze/download: $(CODE_REVIEWER_LOCATION)

analyze/merge: $(CODE_REVIEWER_LOCATION)
	${SCRIPTS_BASE}/analyze.sh ${FROM}

analyze/pull-request: $(CODE_REVIEWER_LOCATION)
	${SCRIPTS_BASE}/analyze.sh --dry-run ${FROM}

.PHONY: release  $(addsuffix /release,$(MAKEDIRS)) all $(addsuffix /all,$(MAKEDIRS)) clean $(addsuffix /clean,$(MAKEDIRS))
