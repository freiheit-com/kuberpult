VERSION=0.0.1

include ../../Makefile.variables

IMAGENAME?=$(IMAGE_REGISTRY)/kuberpult-builder:${VERSION}

build: login
	docker build -t $(IMAGENAME) .

build-pr: build login
	echo "Building on pull request"

build-main: build
	echo "Building on main"

login:
	docker login -u _json_key -p "$(cat ../../credential.json)" europe-west3-docker.pkg.dev

release:
	echo "Releasing $(IMAGENAME)"
	docker push $(IMAGENAME)

publish: login release
