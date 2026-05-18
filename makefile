.PHONY: build install test test-coverage test-integration lint fmt clean markdownlint markdownlint-fix setup

ifneq (,$(wildcard .e2e.env))
include .e2e.env
export
endif

BINARY := dtwiz
GO     := go
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
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

test-integration:
	@if [ -z "$(TEST_DT_ENVIRONMENT)" ]; then echo "ERROR: TEST_DT_ENVIRONMENT is not set" >&2; exit 1; fi
	@if [ -z "$(TEST_DT_ACCESS_TOKEN)" ]; then echo "ERROR: TEST_DT_ACCESS_TOKEN is not set" >&2; exit 1; fi
	@if [ -z "$(TEST_DT_PLATFORM_TOKEN)" ]; then echo "ERROR: TEST_DT_PLATFORM_TOKEN is not set" >&2; exit 1; fi
	$(GO) test -v -race -tags integration -timeout 5m ./test/e2e/...

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

