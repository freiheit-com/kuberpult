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

include ../../Makefile.variables
MAKEFLAGS += --no-builtin-rules

top_dir = \
	$(if $(wildcard $1/.git), \
		$1, \
		$(call top_dir,$(patsubst %/,%,$(dir $1))))
TOP_DIR := $(realpath $(call top_dir,$(CURDIR)))
REL_DIR := $(subst $(TOP_DIR)/,,$(CURDIR))

# Run from container image, as recommended by https://github.com/helm/chart-testing#installation
# `ct` analyses the Git history, so we need to mount the whole repository.
CT := docker run --mount=type=bind,source=$(TOP_DIR),target=/src,readonly --workdir=/src/$(REL_DIR) quay.io/helmpack/chart-testing:v3.5.0 ct
# Manual / local installation
#CT := ct

TGZ_FILE := kuberpult-$(VERSION).tgz

# Chart.yaml: Chart.yaml.tpl
# 	env VERSION="$(VERSION)" envsubst < $< > $@

Chart.yaml:
	earthly +chart.yaml --VERSION=$(VERSION)

$(TGZ_FILE):
	earthly +chart-tarball --VERSION=$(VERSION)

ci/test-values.yaml:
	earthly +test-values.yaml --VERSION=$(VERSION)

ct-test:
ifeq ($(CI),true)
	@echo "running on CI no need to test this again! Check chart testing action."
else
	earthly +ct-test --VERSION=$(VERSION)
endif

test-helm:
	earthly +test-helm --VERSION=$(VERSION)

test-ci:
	earthly +test-ci --VERSION=$(VERSION)

test:
	earthly +test --VERSION=$(VERSION)

clean:
	earthly +clean --VERSION=$(VERSION)

release-tag: $(TGZ_FILE)
	echo "Creating release via git tag pipeline"

.PHONY: clean

all: Chart.yaml
