
# NOTE: this will be replaced with the etf-golang makefile
include ../Makefile.variables
MAKEFLAGS += --no-builtin-rules

export CGO_ENABLED=1

GO := go

GO_FILES := $(shell find . -type f -name '*.go')

test: $(GO_FILES)
	$(GO) test  ./...

all: test

release: #no-op
clean: #no op

.PHONY: all clean release test
