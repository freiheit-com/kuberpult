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

MIN_COVERAGE=74.2
MAKEFLAGS += --no-builtin-rules

include ../../infrastructure/make/go/include.mk

test: unit-test

build-pr: IMAGE_TAG=pr-$(VERSION)
build-pr: lint unit-test bench-test docker release

build-main: IMAGE_TAG=main-$(VERSION)
build-main: lint unit-test bench-test docker release
