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

include ../../Makefile.variables
MAKEFLAGS += --no-builtin-rules

TGZ_FILE := kuberpult-$(VERSION).tgz

Chart.yaml:
	earthly +chart.yaml --VERSION=$(VERSION)

$(TGZ_FILE):
	earthly +chart-tarball --VERSION=$(VERSION)

ci/test-values.yaml:
	earthly +test-values.yaml

ct-test:
ifeq ($(CI),true)
	@echo "running on CI no need to test this again! Check chart testing action."
else
	earthly +ct-test --VERSION=$(VERSION)
endif

test-helm:
	earthly +test-helm --VERSION=$(VERSION)

test:
	earthly +test --VERSION=$(VERSION)

clean:
	earthly +clean --VERSION=$(VERSION)

release-tag: $(TGZ_FILE)
	echo "Creating release via git tag pipeline"

.PHONY: clean

all: Chart.yaml
