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
CONTEXT?=../../
SKIP_TRIVY?=0
SKIP_RETAG_MAIN_AS_PR?=0
SKIP_BUILDER?=0
MAKEFLAGS += --no-builtin-rules
ABS_ROOT_DIR=$(shell git rev-parse --show-toplevel)
MIGRATION_VOLUME="$(ABS_ROOT_DIR)/database/migrations:/kp/database/migrations"
PKG_VOLUME?=-v $(ABS_ROOT_DIR)/pkg:/kp/pkg

.PHONY: compile
compile:
	docker run -w $(SERVICE_DIR) --rm  -v ".:$(SERVICE_DIR)" $(PKG_VOLUME) $(BUILDER_IMAGE) sh -c 'test -n "$(MAIN_PATH)" || exit 0; cd $(MAIN_PATH) && CGO_ENABLED=$(CGO_ENABLED) GOOS=linux go build -o bin/main . && cd ../.. && if [ "$(CGO_ENABLED)" = "1" ]; then ldd $(MAIN_PATH)/bin/main | tr -s [:blank:] '\n' | grep ^/ | xargs -I % install -D % $(MAIN_PATH)/%; fi'

.PHONY: unit-test
unit-test:
	cd $(ABS_ROOT_DIR)/pkg && buf generate
	oapi-codegen -generate "std-http-server" -o $(ABS_ROOT_DIR)/pkg/publicapi/server-gen.go -package publicapi $(ABS_ROOT_DIR)/pkg/publicapi/api.yaml
	docker compose -f $(ROOT_DIR)/docker-compose-unittest.yml up -d
	docker run --rm -w $(SERVICE_DIR) --network host -v ".:$(SERVICE_DIR)" -v $(MIGRATION_VOLUME) $(PKG_VOLUME) $(BUILDER_IMAGE) sh -c "go test $(GO_TEST_ARGS) ./... -coverprofile=coverage.out && go tool cover -html=coverage.out -o coverage.html"
	$(ROOT_DIR)/infrastructure/coverage/check-coverage-go.sh coverage.out $(MIN_COVERAGE) $(SERVICE)
	docker compose -f $(ROOT_DIR)/docker-compose-unittest.yml down

.PHONY: bench-test
bench-test:
	docker compose -f $(ROOT_DIR)/docker-compose-unittest.yml up -d
	docker run --rm -w $(SERVICE_DIR) --network host -v ".:$(SERVICE_DIR)" -v $(MIGRATION_VOLUME) $(PKG_VOLUME) $(BUILDER_IMAGE) sh -c "go test $(GO_TEST_ARGS) -bench=. ./..."
	docker compose -f $(ROOT_DIR)/docker-compose-unittest.yml down

.PHONY: lint
lint:
	$(MAKE) -C $(ROOT_DIR)/pkg gen
	docker run --rm -w /kp/ -v $(shell pwd)/$(ROOT_DIR):/kp/ $(PKG_VOLUME) $(BUILDER_IMAGE) sh -c 'GOFLAGS="-buildvcs=false" golangci-lint run --timeout=15m -j4 --tests=false --skip-files=".*\.pb\.go$$" $(SERVICE_DIR)/...'

.PHONY: docker
docker: compile
	mkdir -p $(MAIN_PATH)/lib
	mkdir -p $(MAIN_PATH)/usr
	test -n "$(MAIN_PATH)" || exit 0; docker build -f Dockerfile --build-arg BUILDER_IMAGE_TAG=$(IMAGE_TAG) $(CONTEXT) -t $(IMAGE_NAME)

.PHONY: release
release:
	test -n "$(MAIN_PATH)" || exit 0; docker push $(IMAGE_NAME)

release-main:
	@echo "Tagging the PR image as main image"
	test -n "$(MAIN_PATH)" || exit 0; docker tag $(IMAGE_NAME) $(MAIN_IMAGE_NAME); docker push $(MAIN_IMAGE_NAME)

retag-main: IMAGE_TAG=pr-$(VERSION)
retag-main:
ifeq ($(SKIP_RETAG_MAIN_AS_PR),1)
	@echo "Skipping retag-main"
else
	@echo "Starting retag-main: Tagging the main image as PR image"
	test -n "$(MAIN_PATH)" || exit 0; docker pull $(MAIN_IMAGE_NAME) && docker tag $(MAIN_IMAGE_NAME) $(IMAGE_NAME) && docker push $(IMAGE_NAME)
endif


trivy-scan: release
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
build-pr: BUILDER_IMAGE=$(DOCKER_REGISTRY_URI)/infrastructure/docker/builder:pr-$(VERSION)
build-pr: lint unit-test bench-test docker release trivy-scan

build-main: IMAGE_TAG=main-$(VERSION)
build-main: lint unit-test bench-test docker release-main trivy-scan
