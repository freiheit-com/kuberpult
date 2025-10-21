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

MIN_COVERAGE=37.0
MAKEFLAGS += --no-builtin-rules
CONTEXT=../..
# avoid calling docker to run buf:
PKG_WITHOUT_DOCKER=1
MAX_DOCKER_SIZE_MB=43
include ../../infrastructure/make/go/include.mk

gen-api:
	pnpm i
	mkdir -p src/api
	buf generate --path api --timeout 5m
