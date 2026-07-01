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


MAX_DOCKER_SIZE_MB=1367
include ../../../Makefile.variables

BUILDER_IMAGENAME_VERSIONED=$(DOCKER_REGISTRY_URI)/infrastructure/docker/builder:$(VERSION)
BUILDER_IMAGENAME_VERSIONED_PR=$(DOCKER_REGISTRY_URI)/infrastructure/docker/builder:pr-$(VERSION)
BUILDER_IMAGENAME_VERSIONED_MAIN=$(DOCKER_REGISTRY_URI)/infrastructure/docker/builder:main-$(VERSION)
BUILDER_IMAGENAME_UNVERSIONED_MAIN=$(DOCKER_REGISTRY_URI)/infrastructure/docker/builder:main

IMAGENAME?=$(DOCKER_REGISTRY_URI)/infrastructure/docker/builder:$(IMAGE_TAG)

build:
	docker build -t $(IMAGENAME) -f Dockerfile ../../../
	../../../infrastructure/scripts/check-docker-size.sh $(IMAGENAME) $(MAX_DOCKER_SIZE_MB) "builder"

build-pr: build publish
	echo "Building on pull request"
	docker tag $(IMAGENAME) $(BUILDER_IMAGENAME_VERSIONED_PR)
	docker push $(BUILDER_IMAGENAME_VERSIONED_PR)

build-main: build publish
	docker tag $(IMAGENAME) $(BUILDER_IMAGENAME_VERSIONED_MAIN)
	docker push $(BUILDER_IMAGENAME_VERSIONED_MAIN)
ifndef SKIP_UNVERSIONED_BUILDS
	docker tag $(IMAGENAME) $(BUILDER_IMAGENAME_UNVERSIONED_MAIN)
	docker push $(BUILDER_IMAGENAME_UNVERSIONED_MAIN)
endif

.PHONY: cleanup-pr
cleanup-pr:
	echo "Nothing to clean"

.PHONY: cleanup-main
cleanup-main:
	echo "Nothing to clean"

release:
	echo "Releasing $(IMAGENAME)"
	docker tag $(IMAGENAME) $(BUILDER_IMAGENAME_VERSIONED)
	docker push $(IMAGENAME)
	docker push $(BUILDER_IMAGENAME_VERSIONED)

publish: release
	echo $(IMAGE_TAG)
	echo $(IMAGENAME)

.PHONY: get-builder-image
get-builder-image:
	@echo "$(KUBERPULT_BUILDER)"

retag-main:
	docker pull $(BUILDER_IMAGENAME_UNVERSIONED_MAIN) && docker tag $(BUILDER_IMAGENAME_UNVERSIONED_MAIN) $(BUILDER_IMAGENAME_VERSIONED_PR) && docker push $(BUILDER_IMAGENAME_VERSIONED_PR)
