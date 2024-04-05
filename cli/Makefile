VERSION    = $(shell git describe --always --long --tags || echo 0.0.1)
IMAGE_NAME = kuberpult-client:$(VERSION)

build-pr:
	docker build . --tag=$(IMAGE_NAME) --build-arg PARENT_CONTAINER=latest
	docker push $(IMAGE_NAME)