
include ../../../Makefile.variables

.PHONY: check-license
check-license:
	@make -C ../../.. check-license

.PHONY: build-main
build-main: check-license
	echo "on main"

.PHONY: build-pr
build-pr: check-license
	echo "on pull request"

.PHONY: cleanup-pr
cleanup-pr:
	echo "Nothing to clean"

.PHONY: cleanup-main
cleanup-main:
	echo "Nothing to clean"


.PHONY: get-builder-image
get-builder-image:
	@echo "$(KUBERPULT_BUILDER)"
