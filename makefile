.PHONY: build install test test-coverage lint clean markdownlint markdownlint-fix

BINARY := dtwiz
GO     := go
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
MD_LINT_CLI_IMAGE := "ghcr.io/igorshubovych/markdownlint-cli:v0.31.1"

build:
	$(GO) build -ldflags "-X github.com/dynatrace-oss/dtwiz/cmd.Version=$(VERSION)" -o $(BINARY) .

install:
	$(GO) install .

COVERAGE_THRESHOLD ?= 20

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

lint:
	golangci-lint run ./...

clean:
	rm -f $(BINARY)

markdownlint:
	docker run -v $(CURDIR):/workdir --rm  $(MD_LINT_CLI_IMAGE)  "**/*.md"

markdownlint-fix:
	docker run -v $(CURDIR):/workdir --rm  $(MD_LINT_CLI_IMAGE)  "**/*.md" --fix

