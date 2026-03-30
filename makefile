.PHONY: build install test lint cleanmarkdownlint markdownlint-fix

BINARY := dtwiz
GO     := go
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
MD_LINT_CLI_IMAGE := "ghcr.io/igorshubovych/markdownlint-cli:v0.31.1"

build:
	$(GO) build -ldflags "-X github.com/dynatrace-oss/dtwiz/cmd.Version=$(VERSION)" -o $(BINARY) .

install:
	$(GO) install .

test:
	$(GO) test ./... -coverprofile=coverage.out
	$(GO) tool cover -func=coverage.out

lint:
	golangci-lint run ./...

clean:
	rm -f $(BINARY)

markdownlint:
	docker run -v $(CURDIR):/workdir --rm  $(MD_LINT_CLI_IMAGE)  "**/*.md"

markdownlint-fix:
	docker run -v $(CURDIR):/workdir --rm  $(MD_LINT_CLI_IMAGE)  "**/*.md" --fix

