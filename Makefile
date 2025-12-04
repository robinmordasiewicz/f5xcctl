# f5xcctl CLI Makefile - F5 Distributed Cloud Control

# Build variables
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo "none")
DATE ?= $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")
LDFLAGS := -ldflags "-s -w -X main.version=$(VERSION) -X main.commit=$(COMMIT) -X main.date=$(DATE)"

# Go variables
GOCMD := go
GOBUILD := $(GOCMD) build
GOTEST := $(GOCMD) test
GOVET := $(GOCMD) vet
GOFMT := $(GOCMD) fmt
GOLINT := golangci-lint

# Binary name
BINARY_NAME := f5xcctl

# Directories
CMD_DIR := ./cmd/f5xcctl
BUILD_DIR := ./build
DIST_DIR := ./dist

.PHONY: all build clean test test-unit test-integration lint fmt vet install generate help check run release release-snapshot build-all build-linux build-darwin build-windows

## help: Print this help message
help:
	@echo "Usage: make [target]"
	@echo ""
	@echo "Targets:"
	@sed -n 's/^##//p' $(MAKEFILE_LIST) | column -t -s ':' | sed -e 's/^/ /'

## all: Build the CLI
all: build

## build: Build the CLI binary
build:
	$(GOBUILD) $(LDFLAGS) -o $(BINARY_NAME) $(CMD_DIR)

## build-all: Build for all platforms
build-all: build-linux build-darwin build-windows

## build-linux: Build for Linux
build-linux:
	GOOS=linux GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-linux-amd64 $(CMD_DIR)
	GOOS=linux GOARCH=arm64 $(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-linux-arm64 $(CMD_DIR)

## build-darwin: Build for macOS
build-darwin:
	GOOS=darwin GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-darwin-amd64 $(CMD_DIR)
	GOOS=darwin GOARCH=arm64 $(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-darwin-arm64 $(CMD_DIR)

## build-windows: Build for Windows
build-windows:
	GOOS=windows GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-windows-amd64.exe $(CMD_DIR)

## install: Install the CLI to GOPATH/bin
install:
	$(GOCMD) install $(LDFLAGS) $(CMD_DIR)

## test: Run tests
test:
	$(GOTEST) -v -race -coverprofile=coverage.out ./...

## test-unit: Run unit tests only (no integration)
test-unit:
	$(GOTEST) -v -race ./internal/...

## test-integration: Run integration tests (requires F5XC_API_URL etc)
test-integration:
	@if [ -z "$$F5XC_API_URL" ]; then \
		echo "Integration tests require environment variables:"; \
		echo "  F5XC_API_URL - API base URL (required)"; \
		echo "  F5XC_API_P12_FILE - Path to P12 certificate file"; \
		echo "  F5XC_P12_PASSWORD - P12 password"; \
		echo "  OR"; \
		echo "  F5XC_API_TOKEN - API token"; \
		exit 1; \
	fi
	$(GOTEST) -v ./integration/...

## test-short: Run short tests
test-short:
	$(GOTEST) -v -short ./...

## coverage: Show test coverage
coverage: test
	$(GOCMD) tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report: coverage.html"

## lint: Run linter
lint:
	$(GOLINT) run ./...

## fmt: Format code
fmt:
	$(GOFMT) ./...

## vet: Run go vet
vet:
	$(GOVET) ./...

## clean: Clean build artifacts
clean:
	rm -f $(BINARY_NAME)
	rm -rf $(BUILD_DIR)
	rm -rf $(DIST_DIR)
	rm -f coverage.out coverage.html

## deps: Download dependencies
deps:
	$(GOCMD) mod download
	$(GOCMD) mod tidy

## generate: Generate code from OpenAPI specs
generate:
	@echo "Code generation from OpenAPI specs..."
	@./scripts/generate.sh

## docs: Generate CLI documentation
docs:
	@mkdir -p docs/reference
	./$(BINARY_NAME) docs generate --format markdown --output docs/reference/

## completion: Generate shell completion scripts
completion:
	@mkdir -p completions
	./$(BINARY_NAME) completion bash > completions/$(BINARY_NAME).bash
	./$(BINARY_NAME) completion zsh > completions/_$(BINARY_NAME)
	./$(BINARY_NAME) completion fish > completions/$(BINARY_NAME).fish
	./$(BINARY_NAME) completion powershell > completions/$(BINARY_NAME).ps1

## release: Build release artifacts using goreleaser
release:
	goreleaser release --clean

## release-snapshot: Build snapshot release (no publish)
release-snapshot:
	goreleaser release --snapshot --clean

## check: Run all checks (fmt, vet, lint, test)
check: fmt vet lint test

## run: Run the CLI (pass ARGS to add arguments)
run: build
	./$(BINARY_NAME) $(ARGS)

# Default target
.DEFAULT_GOAL := help
