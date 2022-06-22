#This file is part of kuberpult.

#Kuberpult is free software: you can redistribute it and/or modify
#it under the terms of the GNU General Public License as published by
#the Free Software Foundation, either version 3 of the License, or
#(at your option) any later version.

#Kuberpult is distributed in the hope that it will be useful,
#but WITHOUT ANY WARRANTY; without even the implied warranty of
#MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
#GNU General Public License for more details.

#You should have received a copy of the GNU General Public License
#along with kuberpult.  If not, see <http://www.gnu.org/licenses/>.

#Copyright 2021 freiheit.com
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
