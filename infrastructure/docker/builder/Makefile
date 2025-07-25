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


include ../../../Makefile.variables

IMAGENAME?=$(DOCKER_REGISTRY_URI)/infrastructure/docker/builder:$(IMAGE_TAG)
IMAGENAME_VERSIONED=$(DOCKER_REGISTRY_URI)/infrastructure/docker/builder:$(VERSION)
IMAGENAME_VERSIONED_PR=$(DOCKER_REGISTRY_URI)/infrastructure/docker/builder:pr-$(VERSION)
IMAGENAME_VERSIONED_MAIN=$(DOCKER_REGISTRY_URI)/infrastructure/docker/builder:main-$(VERSION)
IMAGENAME_UNVERSIONED_MAIN=$(DOCKER_REGISTRY_URI)/infrastructure/docker/builder:main

build:
	docker build -t $(IMAGENAME) -f Dockerfile ../../../

build-pr: build publish
	echo "Building on pull request"
	docker tag $(IMAGENAME) $(IMAGENAME_VERSIONED_PR)
	docker push $(IMAGENAME_VERSIONED_PR)

build-main: build publish
	echo "Building on main"
	docker tag $(IMAGENAME) $(IMAGENAME_VERSIONED_MAIN)
	docker push $(IMAGENAME_VERSIONED_MAIN)
	docker tag $(IMAGENAME) $(IMAGENAME_UNVERSIONED_MAIN)
	docker push $(IMAGENAME_UNVERSIONED_MAIN)

.PHONY: cleanup-pr
cleanup-pr:
	echo "Nothing to clean"

.PHONY: cleanup-main
cleanup-main:
	echo "Nothing to clean"

release:
	echo "Releasing $(IMAGENAME)"
	docker tag $(IMAGENAME) $(IMAGENAME_VERSIONED)
	docker push $(IMAGENAME)
	docker push $(IMAGENAME_VERSIONED)

publish: release
	echo $(IMAGE_TAG)
	echo $(IMAGENAME)

.PHONY: get-builder-image
get-builder-image:
	@echo "$(KUBERPULT_BUILDER)"

retag-main:
	docker pull $(IMAGENAME_UNVERSIONED_MAIN) && docker tag $(IMAGENAME_UNVERSIONED_MAIN) $(IMAGENAME_VERSIONED_PR) && docker push $(IMAGENAME_VERSIONED_PR)
