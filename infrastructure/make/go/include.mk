ROOT_DIR?=../..
include $(ROOT_DIR)/Makefile.variables

MAIN_PATH?=cmd/server
export CGO_ENABLED?=1
GO_TEST_ARGS?=
SKIP_LINT_ERRORS?=false
SERVICE?=$(notdir $(shell pwd))
IMAGE_NAME?=$(DOCKER_REGISTRY_URI)/kuberpult-$(SERVICE):$(IMAGE_TAG)
MAIN_IMAGE_NAME=$(DOCKER_REGISTRY_URI)/kuberpult-$(SERVICE):main
SERVICE_DIR?=/kp/services/$(SERVICE)
MIN_COVERAGE?=99.9 # should be overwritten by every service
CONTEXT?=.
SKIP_DEPS?=0
SKIP_TRIVY?=0
MAKEFLAGS += --no-builtin-rules

.PHONY: deps
deps:
ifeq ($(SKIP_DEPS),0)
	@echo "docker build deps image"
	IMAGE_TAG=latest $(MAKE) -C $(ROOT_DIR)/infrastructure/docker/deps build
else
	@echo "Skipping docker build for deps image"
endif

.PHONY: compile
compile: deps
	docker run -w $(SERVICE_DIR) --rm  -v ".:$(SERVICE_DIR)" $(DEPS_IMAGE) sh -c 'test -n "$(MAIN_PATH)" || exit 0; cd $(MAIN_PATH) && CGO_ENABLED=$(CGO_ENABLED) GOOS=linux go build -o bin/main . && cd ../.. && if [ "$(CGO_ENABLED)" = "1" ]; then ldd $(MAIN_PATH)/bin/main | tr -s [:blank:] '\n' | grep ^/ | xargs -I % install -D % $(MAIN_PATH)/%; fi'

.PHONY: unit-test
unit-test: deps
	docker compose -f $(ROOT_DIR)/docker-compose-unittest.yml up -d
	docker run --rm -w $(SERVICE_DIR) --network host -v ".:$(SERVICE_DIR)" $(DEPS_IMAGE) sh -c "go test $(GO_TEST_ARGS) ./... -coverprofile=coverage.out && go tool cover -html=coverage.out -o coverage.html"
	$(ROOT_DIR)/infrastructure/coverage/check-coverage-go.sh coverage.out $(MIN_COVERAGE) $(SERVICE)
	docker compose -f $(ROOT_DIR)/docker-compose-unittest.yml down

.PHONY: bench-test
bench-test: deps
	docker compose -f $(ROOT_DIR)/docker-compose-unittest.yml up -d
	docker run --rm -w $(SERVICE_DIR) --network host -v ".:$(SERVICE_DIR)" $(DEPS_IMAGE) sh -c "go test $(GO_TEST_ARGS) -bench=. ./..."
	docker compose -f $(ROOT_DIR)/docker-compose-unittest.yml down

.PHONY: lint
lint: deps
	docker run --rm -w $(SERVICE_DIR) -v ".:$(SERVICE_DIR)" $(DEPS_IMAGE) sh -c 'golangci-lint run --timeout=15m -j4 --tests=false --skip-files=".*\.pb\.go$$" ./... || $(SKIP_LINT_ERRORS) && exhaustruct -test=false $$(go list ./... | grep -v "github.com/freiheit-com/kuberpult/pkg/api" | grep -v "github.com/freiheit-com/kuberpult/pkg/publicapi")'

.PHONY: docker
docker: compile
	mkdir -p $(MAIN_PATH)/lib
	mkdir -p $(MAIN_PATH)/usr
	test -n "$(MAIN_PATH)" || exit 0; docker build -f Dockerfile $(CONTEXT) -t $(IMAGE_NAME)

.PHONY: release
release:
	test -n "$(MAIN_PATH)" || exit 0; docker push $(IMAGE_NAME)
	test -n "$(MAIN_PATH)" || exit 0; docker tag $(IMAGE_NAME) $(MAIN_IMAGE_NAME); docker push $(MAIN_IMAGE_NAME)

trivy-scan: docker
ifeq ($(SKIP_TRIVY),1)
	@echo "Skipping trivy"
else
	@echo "Starting trivy check for $(IMAGE_NAME)"
	KUBERPULT_SERVICE_IMAGE=$(IMAGE_NAME) $(MAKE) -C $(ROOT_DIR)/trivy scan-service-pr
endif

.PHONY: datadog-wrapper
datadog-wrapper:
	docker run --rm -v "datadog-init:/datadog-init" datadog/serverless-init:1-alpine

test: unit-test

build-pr: IMAGE_TAG=pr-$(VERSION)
build-pr: lint unit-test bench-test docker release trivy-scan

build-main: IMAGE_TAG=main-$(VERSION)
build-main: lint unit-test bench-test docker release trivy-scan
