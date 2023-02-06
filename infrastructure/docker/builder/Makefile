
include ../../../Makefile.variables

IMAGENAME?=$(IMAGE_REGISTRY)/kuberpult-builder:${VERSION}

build:
	docker build -t $(IMAGENAME) .

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
	docker push $(IMAGENAME)

publish: release
	echo $(VERSION)
	echo $(IMAGENAME)

.PHONY: get-builder-image
get-builder-image:
	@echo "$(KUBERPULT_BUILDER)"
