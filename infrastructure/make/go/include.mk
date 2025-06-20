include ../../Makefile.variables

GOARCH?=arm64
MAIN_PATH?=cmd/server
CGO_ENABLED?=1
GO_TEST_ARGS?=
SKIP_LINT_ERRORS?=false
SERVICE?=$(notdir $(shell pwd))
IMAGE_NAME?=$(DOCKER_REGISTRY_URI)/$(SERVICE):$(VERSION)
SERVICE_DIR?=/kp/services/$(SERVICE)

.PHONY: deps
deps:
	make -C ../../infrastructure/docker/deps build 

.PHONY: compile
compile: deps
	docker run -w $(SERVICE_DIR) --rm  -v ".:$(SERVICE_DIR)" $(DEPS_IMAGE) sh -c 'cd $(MAIN_PATH) &&  CGO_ENABLED=$(CGO_ENABLED) GOARCH="$(GOARCH)" GOOS=linux go build -o bin/main . && cd ../.. && if [ "$(CGO_ENABLED)" = "1" ]; then ldd $(MAIN_PATH)/bin/main | tr -s [:blank:] '\n' | grep ^/ | xargs -I % install -D % $(MAIN_PATH)/%; fi'

.PHONY: unit-test
unit-test: deps
	docker compose -f ../../docker-compose-unittest.yml up -d
	docker run --rm -w $(SERVICE_DIR) --network host -v ".:$(SERVICE_DIR)" $(DEPS_IMAGE) sh -c "go test $(GO_TEST_ARGS) ./... -coverprofile=coverage.out && go tool cover -html=coverage.out -o coverage.html"
	docker compose -f ../../docker-compose-unittest.yml down

.PHONY: bench-test
bench-test: deps
	docker compose -f ../../docker-compose-unittest.yml up -d
	docker run --rm -w $(SERVICE_DIR) --network host -v ".:$(SERVICE_DIR)" $(DEPS_IMAGE) sh -c "go test $(GO_TEST_ARGS) -bench=. ./..."
	docker-compose -f ../../docker-compose-unittest.yml down

.PHONY: lint
lint: deps
	docker run --rm -w $(SERVICE_DIR) -v ".:$(SERVICE_DIR)" $(DEPS_IMAGE) sh -c 'golangci-lint run --timeout=15m -j4 --tests=false ./... || $(SKIP_LINT_ERRORS) && exhaustruct -test=false $$(go list ./... | grep -v "github.com/freiheit-com/kuberpult/pkg/api" | grep -v "github.com/freiheit-com/kuberpult/pkg/publicapi")'

.PHONY: docker
docker: artifacts compile
	docker build . -t $(IMAGE_NAME)


.PHONY: release
release:
	docker push $(IMAGE_NAME)

.PHONY: datadog-wrapper
datadog-wrapper:
	docker run --rm -v "datadog-init:/datadog-init" datadog/serverless-init:1-alpine
