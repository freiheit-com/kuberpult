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
	echo "Publishing $(IMAGENAME)"
	docker login -u _json_key -p "$(cat credential.json)" europe-west3-docker.pkg.dev
	docker push $(IMAGENAME)
