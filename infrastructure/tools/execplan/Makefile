IMAGE_NAME:=images/execution-plan
GO_MAINPATH    ?= main.go
GO_BIN		?= ./bin/main
GO_COVERAGE_THRESHOLD=80.0

#include ../../make/go/include.mk

check:
	echo "nothing to check"

exec-plan-compile:
	go build -o plan $(GO_MAINPATH)

local-bin:
	@go build \
		-o $(GO_BIN) \
		./$(GO_MAINPATH)

check:
	echo "nothing to check"

.PHONY: coverage
coverage:
	go test -coverprofile=coverage.out ./...
	go tool cover -func=coverage.out
	go tool cover -html=coverage.out


