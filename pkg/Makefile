#This file is part of kuberpult.

#Kuberpult is free software: you can redistribute it and/or modify
#it under the terms of the Expat(MIT) License as published by
#the Free Software Foundation.

#Kuberpult is distributed in the hope that it will be useful,
#but WITHOUT ANY WARRANTY; without even the implied warranty of
#MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
#MIT License for more details.

#You should have received a copy of the MIT License
#along with kuberpult. If not, see <https://directory.fsf.org/wiki/License:Expat>.

#Copyright 2023 freiheit.com

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
