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
ifeq ($(IMAGE_TAG),latest)
	make -C ../../.. builder
endif
	docker build -t $(DEPS_IMAGE) -f Dockerfile ../../.. --build-arg BUILDER_IMAGE=$(BUILDER_IMAGE)
