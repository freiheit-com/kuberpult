include ../../../Makefile.variables
.PHONY: build
build:
	make -C ../../.. builder
	docker build -t $(DEPS_IMAGE) -f Dockerfile ../../..