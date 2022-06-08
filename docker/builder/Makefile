VERSION=0.0.1

include ../../Makefile.variables

IMAGENAME?=$(IMAGE_REGISTRY)/kuberpult-builder:${VERSION}

build:
	docker build -t $(IMAGENAME) .

build-pr: build
	echo "Building on pull request"

build-main: build
	echo "Building on main"

publish:
	docker push -t $(IMAGENAME)
