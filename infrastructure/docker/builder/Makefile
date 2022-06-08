VERSION=0.0.1

include ../../Makefile.variables

IMAGENAME?=$(IMAGE_REGISTRY)/kuberpult-builder:${VERSION}

build: 
	docker build -t $(IMAGENAME) .

build-pr: build 
	echo "Building on pull request"

build-main: build
	echo "Building on main"

login:
	cat ../../credential.json | docker login -u _json_key --password-stdin europe-west3-docker.pkg.dev

release:
	echo "Releasing $(IMAGENAME)"
	docker push $(IMAGENAME)

publish: login release
