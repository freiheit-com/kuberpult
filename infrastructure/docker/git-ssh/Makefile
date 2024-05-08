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

include ../../../Makefile.variables

IMAGENAME?=$(DOCKER_REGISTRY_URI)/infrastructure/docker/git-ssh:${IMAGE_TAG}

build:
	earthly +build --DOCKER_REGISTRY_URI=$(DOCKER_REGISTRY_URI) --IMAGE_TAG=$(IMAGE_TAG)

build-pr: build publish
	echo "Building on pull request"

build-main: build publish
	echo "Building on main"

.PHONY: cleanup-pr
cleanup-pr:
	echo "Nothing to clean"

.PHONY: cleanup-main
cleanup-main:
	echo "Nothing to clean"

release:
	echo "Releasing $(IMAGENAME)"
	earthly --no-output --push +publish --DOCKER_REGISTRY_URI=$(DOCKER_REGISTRY_URI) --IMAGE_TAG=$(IMAGE_TAG)

publish: release
	echo $(IMAGE_TAG)
	echo $(IMAGENAME)

.PHONY: get-builder-image
get-builder-image:
	@echo "$(KUBERPULT_BUILDER)"
