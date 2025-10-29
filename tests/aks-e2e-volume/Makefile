SHELL := /usr/bin/env bash
.SHELLFLAGS := -euo pipefail -c

TEST_NAME := $(notdir $(patsubst %/,%,$(CURDIR)))
TEST_ENV_FILES := test.env test-seed.env $(if $(wildcard test-local.env), test-local.env)
TEST_DEPENDENCIES := kompoxops kompoxapp.yml $(TEST_ENV_FILES)

define TEST_ENV_SCRIPT
set -a; \
for i in $(TEST_ENV_FILES); do \
	if test -r $$i; then . $$i; fi; \
done; \
set +a
endef

define TEST_RUN_SCRIPT
for i in $(1) $(2); do \
	if test ! -r $$i; then \
		echo "Skipping: $$i is missing" >&2; \
		exit 0; \
	fi; \
done; \
$(TEST_ENV_SCRIPT); \
echo "Running $(1)" >&2; \
$(1)
endef

TOPDIR ?= $(shell d=$(CURDIR); while test $$d != /; do test -f $$d/go.mod && echo $$d && exit 0 || d=$$(dirname $$d); done)

.PHONY: setup run teardown clean all

default:

all:
	$(MAKE) setup
	$(MAKE) run
	$(MAKE) teardown

test-seed.env:
	@echo "TEST_NAME=$(TEST_NAME)" > $@
	@echo "SERVICE_NAME=$(TEST_NAME)-$(shell date +%Y%m%d-%H%M%S)" >> $@
	@$(call TEST_RUN_SCRIPT,./test-seed.sh) >> $@

kompoxops:
	$(MAKE) -C $(TOPDIR) cmd
	cp $(TOPDIR)/kompoxops .

kompoxapp.yml: kompoxapp.yml.in $(TEST_ENV_FILES)
	@$(TEST_ENV_SCRIPT) && echo "Creating $@ from $<" && envsubst < $< > $@

setup: $(TEST_DEPENDENCIES)
	@$(call TEST_RUN_SCRIPT,./test-setup.sh,$(TEST_DEPENDENCIES))

run: $(TEST_DEPENDENCIES)
	@$(call TEST_RUN_SCRIPT,./test-run-disk.sh,$(TEST_DEPENDENCIES))
	@$(call TEST_RUN_SCRIPT,./test-run-files.sh,$(TEST_DEPENDENCIES))

teardown:
	@$(call TEST_RUN_SCRIPT,./test-teardown.sh,$(TEST_DEPENDENCIES))

clean:
	@$(call TEST_RUN_SCRIPT,./test-clean.sh)
	rm -rf kompoxops kompoxops.yml
