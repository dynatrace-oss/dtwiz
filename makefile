.PHONY: build install test test-coverage test-integration lint fmt clean markdownlint markdownlint-fix setup

ifneq (,$(wildcard .e2e.env))
include .e2e.env
export
endif

BINARY := dtwiz
GO     := go
VERSION ?= dev
MD_LINT_CLI_IMAGE := "ghcr.io/igorshubovych/markdownlint-cli:v0.31.1"

build:
	$(GO) build -ldflags "-X github.com/dynatrace-oss/dtwiz/pkg/version.Version=$(VERSION)" -o $(BINARY) .

install:
	$(GO) install .

COVERAGE_THRESHOLD ?= 30

test:
	$(GO) test ./pkg/... -coverprofile=coverage.out
	$(GO) tool cover -func=coverage.out

# Run tests and enforce coverage threshold
test-coverage:
	@echo "Running tests with coverage..."
	@$(GO) test -race -coverprofile=coverage.out -covermode=atomic ./pkg/...
	@echo ""
	@echo "=== Package Coverage ==="
	@$(GO) tool cover -func=coverage.out | grep -E "^(total|.*\t)" | tail -30
	@echo ""
	@COVERAGE=$$($(GO) tool cover -func=coverage.out | grep total | awk '{print $$3}' | sed 's/%//'); \
	COVERAGE_INT=$${COVERAGE%.*}; \
	echo "Total coverage: $${COVERAGE}% (threshold: $(COVERAGE_THRESHOLD)%)"; \
	if [ "$$COVERAGE_INT" -lt "$(COVERAGE_THRESHOLD)" ]; then \
		echo "FAIL: Coverage $${COVERAGE}% is below the $(COVERAGE_THRESHOLD)% threshold"; \
		exit 1; \
	fi; \
	echo "OK: Coverage meets threshold"

fmt:
	@golangci-lint fmt ./...

lint:
	golangci-lint run ./...

ifeq ($(OS),Windows_NT)
RACE_FLAG :=
else
RACE_FLAG := -race
endif

# SEQUENTIAL=true runs integration tests one at a time (default: parallel).
SEQUENTIAL ?= false
ifeq ($(SEQUENTIAL),true)
SEQ_ENV := TEST_SEQUENTIAL=1
else
SEQ_ENV :=
endif

TEST_DEBUG ?= $(DEBUG)
export TEST_DEBUG

test-integration: SHELL := /bin/bash
test-integration:
ifeq ($(strip $(TEST_DT_ENVIRONMENT)),)
	$(error TEST_DT_ENVIRONMENT is not set)
endif
ifeq ($(strip $(TEST_DT_PLATFORM_TOKEN)),)
	$(error TEST_DT_PLATFORM_TOKEN is not set)
endif
	@set -o pipefail; \
	LOG=$$(mktemp); \
	trap 'rm -f "$$LOG"' EXIT; \
	$(SEQ_ENV) $(GO) test -v -count=1 $(RACE_FLAG) -tags integration -timeout 30m $(if $(RUN),-run $(RUN),) ./test/e2e/... 2>&1 | tee "$$LOG"; \
	status=$$?; \
	echo ""; \
	echo "=== Integration Test Summary ==="; \
	grep -E "^--- (PASS|FAIL|SKIP):" "$$LOG" | sed 's/^--- //' | sort; \
	echo ""; \
	echo "$$(grep -c '^--- PASS:' "$$LOG") passed, $$(grep -c '^--- FAIL:' "$$LOG") failed, $$(grep -c '^--- SKIP:' "$$LOG") skipped"; \
	exit $$status

clean:
	rm -f $(BINARY)

markdownlint:
	docker run -v $(CURDIR):/workdir --rm  $(MD_LINT_CLI_IMAGE)  "**/*.md"

markdownlint-fix:
	docker run -v $(CURDIR):/workdir --rm  $(MD_LINT_CLI_IMAGE)  "**/*.md" --fix

setup:
	git config --local core.hooksPath .githooks
	chmod +x .githooks/* || true
	@echo "Git hooks installed from .githooks/"

