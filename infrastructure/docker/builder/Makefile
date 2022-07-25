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
include ../../../Makefile.variables

IMAGENAME?=$(IMAGE_REGISTRY)/kuberpult-builder:${VERSION}

build: 
	docker build -t $(IMAGENAME) .

build-pr: build publish
	echo "Building on pull request"

build-main: build publish
	echo "Building on main"

login:
	cat ../../../credential.json | docker login -u _json_key --password-stdin $(IMAGE_SERVER)

release:
	echo "Releasing $(IMAGENAME)"
	docker push $(IMAGENAME)

publish: login release
	echo $(VERSION)
	echo $(IMAGENAME)

.PHONY: get-builder-image
get-builder-image:
	@echo ""
