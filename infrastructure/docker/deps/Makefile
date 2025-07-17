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

.PHONY: build
build:
#	make -C ../../.. builder
	docker build -t $(DEPS_IMAGE) -f Dockerfile ../../.. --build-arg BUILDER_IMAGE=$(BUILDER_IMAGE)
	docker tag $(DEPS_IMAGE) $(DEPS_MAIN_IMAGE)

build-main: build
	docker push $(DEPS_MAIN_IMAGE)

build-pr: build