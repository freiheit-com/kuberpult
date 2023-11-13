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
SHELL := sh

include ./Makefile.variables
MAKEFLAGS += --no-builtin-rules

SCRIPTS_BASE:=infrastructure/scripts/make


MAKEDIRS := services/cd-service services/rollout-service services/frontend-service charts/kuberpult pkg/api pkg

export USER_UID := $(shell id -u)
.install:
	touch .install

$(addsuffix /release,$(MAKEDIRS)):
	make -C $(dir $@) release

release: $(addsuffix /release,$(MAKEDIRS))
	git tag $(VERSION)

$(addsuffix /clean,$(MAKEDIRS)):
	make -C $(dir $@) clean

clean: $(addsuffix /clean,$(MAKEDIRS))

$(addsuffix /test,$(MAKEDIRS)):
	make -C $(dir $@) test

test: $(addsuffix /test,$(MAKEDIRS))

$(addsuffix /all,$(MAKEDIRS)):
	make -C $(dir $@) all

plan:
	@infrastructure/scripts/execution-plan/plan-pr.sh

all: $(addsuffix /all,$(MAKEDIRS))

init:

.PHONY: release  $(addsuffix /release,$(MAKEDIRS)) all $(addsuffix /all,$(MAKEDIRS)) clean $(addsuffix /clean,$(MAKEDIRS))

.PHONY: check-license
check-license:
	@bash check.sh || (echo run "bash check.sh" locally, commit the result and push; exit 1)

.PHONY: version
version:
	@echo $(VERSION)

.PHONY: cleanup-pr
cleanup-pr:
	@echo "Nothing to do"

.PHONY: cleanup-main
cleanup-main:
	@echo "Nothing to do"

.PHONY: builder
builder:
	IMAGE_TAG=latest make -C infrastructure/docker/builder build

kuberpult:
	make -C services/frontend-service src/api/api.ts
	docker compose up --build

kuberpult-earthly:
	earthly +all-services --UID=$(USER_UID) --target docker
	docker-compose -f docker-compose-earthly.yml up 

cache:
	earthly --remote-cache=ghcr.io/freiheit-com/kuberpult/kuberpult-frontend-service:cache --push +frontend-service --target release --UID=$(USER_UID)
	earthly --remote-cache=ghcr.io/freiheit-com/kuberpult/kuberpult-cd-service:cache --push +cd-service --UID=$(USER_UID) --target release
	earthly --remote-cache=ghcr.io/freiheit-com/kuberpult/kuberpult-rollout-service:cache --push +rollout-service --UID=$(USER_UID) --target release
