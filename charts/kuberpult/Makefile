#This file is part of kuberpult.

#Kuberpult is free software: you can redistribute it and/or modify
#it under the terms of the GNU General Public License as published by
#the Free Software Foundation, either version 3 of the License, or
#(at your option) any later version.

#Kuberpult is distributed in the hope that it will be useful,
#but WITHOUT ANY WARRANTY; without even the implied warranty of
#MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
#GNU General Public License for more details.

#You should have received a copy of the GNU General Public License
#along with kuberpult.  If not, see <http://www.gnu.org/licenses/>.

#Copyright 2021 freiheit.com
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

Chart.yaml: Chart.yaml.tpl 
	env VERSION=$(VERSION) envsubst < Chart.yaml.tpl > $@

values.yaml: values.yaml.tpl 
	env VERSION=$(VERSION) envsubst < values.yaml.tpl > $@

$(TGZ_FILE): Chart.yaml values.yaml templates/*
	helm package .

ct-test: Chart.yaml values.yaml
ifeq ($(CI),true)
	@echo "running on CI no need to test this again! Check chart testing action."
else
	$(CT) lint --chart-yaml-schema=ci/chart_schema.yaml --lint-conf=ci/lintconf.yaml --target-branch=main --chart-dirs=. --charts=.
endif

test: ct-test $(TGZ_FILE)
	helm lint $(TGZ_FILE) --set git.url=test --set ingress.domainName=kuberpult.example.com
	helm lint $(TGZ_FILE) --set git.url=test --set ingress.domainName=kuberpult.example.com --set ingress.exposeReleaseEndpoint=true

clean:
	rm -f Chart.yaml values.yaml
	rm -f kuberpult-*.tgz

release-tag: test $(TGZ_FILE)
	echo "Creating release via git tag pipeline"

.PHONY: clean

all: Chart.yaml values.yaml
