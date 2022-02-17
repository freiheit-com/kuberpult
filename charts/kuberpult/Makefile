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
MAKEFLAGS += --no-builtin-rules

VERSION=$(shell cat ../../version)
export VERSION

Chart.yaml: Chart.yaml.tpl ../../version
	envsubst < Chart.yaml.tpl > $@

values.yaml: values.yaml.tpl ../../version
	envsubst < values.yaml.tpl > $@

kuberpult-$(VERSION).tgz: Chart.yaml values.yaml templates/*
	helm package .

ct-test:
ifeq ($(CI),true)
	@echo "running on CI no need to test this again! Check chart testing action."
else
	ct lint --target-branch main --all --chart-dirs ../ --chart-yaml-schema ci/chart_schema.yaml --lint-conf ci/lintconf.yaml
endif

test: ct-test kuberpult-$(VERSION).tgz
	helm lint kuberpult-$(VERSION).tgz --set git.url=test --set ingress.domainName=kuberpult.example.com
	helm lint kuberpult-$(VERSION).tgz --set git.url=test --set ingress.domainName=kuberpult.example.com --set ingress.exposeReleaseEndpoint=true

clean:
	rm -f Chart.yaml values.yaml
	rm -f kuberpult-*.tgz

release: test kuberpult-$(VERSION).tgz
	@echo "Please upload cd-$(VERSION).tgz to the github release"
	gsutil cp kuberpult-$(VERSION).tgz gs://fdc-charts
	
.PHONY: clean

all: Chart.yaml values.yaml
