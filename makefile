.PHONY: build install clean test test-coverage test-integration fmt lint markdownlint markdownlint-fix snapshot setup

ifneq (,$(wildcard .e2e.env))
include .e2e.env
export
endif

# --- Variables ---------------------------------------------------------------

GO     := go
BINARY := bin/dtwiz

BUILD_DIR    := build
COVERAGE_OUT := $(BUILD_DIR)/coverage.out

VERSION ?= dev
COMMIT  ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
DATE    ?= $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")

VERSION_PKG := github.com/dynatrace-oss/dtwiz/pkg/version
LDFLAGS     := -ldflags "-X $(VERSION_PKG).Version=$(VERSION) -X $(VERSION_PKG).Commit=$(COMMIT) -X $(VERSION_PKG).Date=$(DATE) -s -w"

# Minimum test coverage percentage. Raise as coverage improves: 65 -> 70 -> 80 -> 85.
COVERAGE_THRESHOLD ?= 65

ifeq ($(OS),Windows_NT)
RACE_FLAG :=
else
RACE_FLAG := -race
endif

# SEQUENTIAL=true runs integration tests one at a time (default: parallel).
SEQUENTIAL ?= false
SEQ_ENV    := $(if $(filter true,$(SEQUENTIAL)),TEST_SEQUENTIAL=1)

TEST_DEBUG ?= $(DEBUG)
export TEST_DEBUG

MD_LINT_CLI_IMAGE := ghcr.io/igorshubovych/markdownlint-cli:v0.49.1
CONTAINER_RUNTIME := $(shell command -v podman 2>/dev/null || command -v docker 2>/dev/null)
MD_LINT            = $(CONTAINER_RUNTIME) run --rm -v $(CURDIR):/workdir $(MD_LINT_CLI_IMAGE) "**/*.md"

# --- Build -------------------------------------------------------------------

build:
	$(GO) build $(LDFLAGS) -o $(BINARY) .

install:
	$(GO) install .

clean:
	rm -f $(BINARY)
	rm -rf $(BUILD_DIR)

# --- Test --------------------------------------------------------------------

test:
	@mkdir -p $(BUILD_DIR)
	$(GO) test ./pkg/... -coverprofile=$(COVERAGE_OUT)
	$(GO) tool cover -func=$(COVERAGE_OUT)

# Run tests and enforce coverage threshold.
test-coverage:
	@echo "Running tests with coverage ..."
	@mkdir -p $(BUILD_DIR)
	@$(GO) test $(RACE_FLAG) -coverprofile=$(COVERAGE_OUT) -covermode=atomic ./pkg/...
	@echo ""
	@echo "=== Package Coverage ==="
	@$(GO) tool cover -func=$(COVERAGE_OUT) | tail -30
	@echo ""
	@COVERAGE=$$($(GO) tool cover -func=$(COVERAGE_OUT) | awk '/^total:/ { sub("%", "", $$3); print $$3 }'); \
	echo "Total coverage: $${COVERAGE}% (threshold: $(COVERAGE_THRESHOLD)%)"; \
	if [ "$${COVERAGE%.*}" -lt "$(COVERAGE_THRESHOLD)" ]; then \
		echo "FAIL: Coverage $${COVERAGE}% is below the $(COVERAGE_THRESHOLD)% threshold"; \
		exit 1; \
	fi; \
	echo "OK: Coverage meets threshold"

test-integration: SHELL := bash
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
	$(SEQ_ENV) $(GO) test -v -count=1 $(RACE_FLAG) -tags integration -timeout 30m $(if $(RUN),-run $(RUN)) ./test/e2e/... 2>&1 | tee "$$LOG"; \
	status=$$?; \
	echo ""; \
	echo "=== Integration Test Summary ==="; \
	grep -E "^--- (PASS|FAIL|SKIP):" "$$LOG" | sed 's/^--- //' | sort; \
	echo ""; \
	echo "$$(grep -c '^--- PASS:' "$$LOG") passed, $$(grep -c '^--- FAIL:' "$$LOG") failed, $$(grep -c '^--- SKIP:' "$$LOG") skipped"; \
	exit $$status

# --- Lint & format -----------------------------------------------------------

fmt:
	@golangci-lint fmt ./...

lint:
	golangci-lint run ./...

markdownlint: require-container-runtime
	$(MD_LINT)

markdownlint-fix: require-container-runtime
	$(MD_LINT) --fix

.PHONY: require-container-runtime
require-container-runtime:
	@test -n "$(CONTAINER_RUNTIME)" || { echo "Error: neither docker nor podman found in PATH"; exit 1; }

# --- Release & repo setup ----------------------------------------------------

snapshot:
	GORELEASER_SNAPSHOT_TAG="snapshot-$$(git rev-parse --abbrev-ref HEAD | tr '/' '-')" \
		goreleaser release --snapshot --clean

setup:
	git config --local core.hooksPath .githooks
	chmod +x .githooks/* || true
	@echo "Git hooks installed from .githooks/"
