
include ./Makefile.variables
MAKEFLAGS += --no-builtin-rules

SCRIPTS_BASE:=infrastructure/scripts/make
CODE_REVIEWER_LOCATION?=$(HOME)/bin/codereviewr


MAKEDIRS := services/cd-service services/frontend-service charts/kuberpult pkg/api pkg


.install:
	touch .install

$(addsuffix /release,$(MAKEDIRS)):
	make -C $(dir $@) release

release: $(addsuffix /release,$(MAKEDIRS))
	git tag $(VERSION)

$(addsuffix /clean,$(MAKEDIRS)):
	make -C $(dir $@) clean

clean: $(addsuffix /clean,$(MAKEDIRS))

$(addsuffix /test,$(MAKEDIRS)):
	make -C $(dir $@) test

test: $(addsuffix /test,$(MAKEDIRS))

$(addsuffix /all,$(MAKEDIRS)):
	make -C $(dir $@) all

plan:
	@infrastructure/scripts/execution-plan/plan-pr.sh

all: $(addsuffix /all,$(MAKEDIRS))

init:

$(CODE_REVIEWER_LOCATION):
ifeq ($(CI),true)
	@wget -O /tmp/codereviewr https://storage.googleapis.com/codereviewr_a7ed108e-470d-4be0-b5bc-001e4d64f0a2/latest/codereviewr
	install -m 755 /tmp/codereviewr $@
else
	@wget -O /tmp/codereviewr https://storage.googleapis.com/codereviewr_a7ed108e-470d-4be0-b5bc-001e4d64f0a2/latest/codereviewr
	install -m 755 /tmp/codereviewr $@
endif

analyze/download: $(CODE_REVIEWER_LOCATION)

analyze/merge: $(CODE_REVIEWER_LOCATION)
	${SCRIPTS_BASE}/analyze.sh ${FROM}

analyze/pull-request: $(CODE_REVIEWER_LOCATION)
	${SCRIPTS_BASE}/analyze.sh --dry-run ${FROM}

.PHONY: release  $(addsuffix /release,$(MAKEDIRS)) all $(addsuffix /all,$(MAKEDIRS)) clean $(addsuffix /clean,$(MAKEDIRS))

.PHONY: check-license
check-license:
	@sh check.sh || (echo run "bash check.sh" locally, commit the result and push; exit 1)

.PHONY: version
version:
	@echo $(VERSION)

.PHONY: cleanup-pr
cleanup-pr:
	@echo "Nothing to do"

.PHONY: cleanup-main
cleanup-main:
	@echo "Nothing to do"
