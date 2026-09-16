LINT_TIMEOUT := 10m
GOLANGCI_VERSION ?= v2.12.2
GOLANGCI ?= go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@$(GOLANGCI_VERSION)
GOVULNCHECK_VERSION ?= latest
GOVULNCHECK ?= go run golang.org/x/vuln/cmd/govulncheck@$(GOVULNCHECK_VERSION)

SHELL = /usr/bin/env bash -o pipefail
.SHELLFLAGS = -ec

.PHONY: all
all: test

.PHONY: fmt
fmt:
	go fmt ./...

.PHONY: test
test:
	go test -v ./...

.PHONY: deps
deps:
	go mod tidy

.PHONY: lint
lint:
	@$(GOLANGCI) run --timeout $(LINT_TIMEOUT)

.PHONY: vulncheck
vulncheck:
	@$(GOVULNCHECK) ./...

.PHONY: check
check: lint vulncheck
