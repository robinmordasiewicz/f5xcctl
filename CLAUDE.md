# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

f5xcctl is a kubectl-style CLI for managing F5 Distributed Cloud (F5XC) resources. It provides CRUD operations for load balancers, origin pools, firewalls, certificates, DNS, and other F5XC resources.

## Build and Development Commands

```bash
# Build
make build              # Build binary (./f5xcctl)
make install            # Install to GOPATH/bin

# Testing
make test               # Run all tests with coverage
make test-unit          # Run unit tests only (./internal/...)
make test-integration   # Run integration tests (requires F5XC credentials)

# Code Quality
make lint               # Run golangci-lint
make fmt                # Format code
make vet                # Run go vet
make check              # Run all: fmt, vet, lint, test

# Run single test
go test -v -run TestName ./internal/cmd/...
```

## Architecture

### Command Structure (Cobra-based)

- **Entry point**: `cmd/f5xcctl/main.go` - minimal, sets version info and calls `cmd.Execute()`
- **Root command**: `internal/cmd/root.go` - defines global flags, initializes config, registers subcommands
- **Verbs**: `internal/cmd/verbs.go` - kubectl-style commands (get, create, delete, apply, describe, replace, label)
- **Resource types**: `internal/cmd/resources.go` - `ResourceRegistry` map defines all F5XC resource types with API paths, aliases, and short names

### Key Patterns

**Resource Resolution**: All resource types are registered in `ResourceRegistry` with canonical names, plurals, short forms, and aliases. Use `ResolveResourceType(name)` to resolve any alias to its `*ResourceType`.

**API Client** (`internal/runtime/client.go`):

- `NewClient()` - creates client from config file + credentials
- `NewClientFromEnv()` - creates client from environment variables (for testing)
- Uses `retryablehttp` with automatic retries on 429, 503, 504
- Authentication via `Authenticator` interface (token, cert, P12)

**Authentication** (`internal/auth/`):

- `TokenAuth` - API token in Authorization header
- `CertAuth` - X.509 client certificate
- `P12Auth` - PKCS#12 certificate bundle
- `BrowserAuth` - OAuth flow with local callback server

**Configuration** (`internal/config/config.go`):

- Config file: `~/.f5xc/config.yaml` (profiles with tenant, API URL, auth method)
- Credentials file: `~/.f5xc/credentials` (API tokens, P12 passwords)

### Adding a New Resource Type

1. Add entry to `ResourceRegistry` in `internal/cmd/resources.go`:

```go
"new_resource": {
    Name:           "new_resource",
    Plural:         "new_resources",
    Short:          "nr",
    APIPath:        "/api/config/namespaces/{namespace}/new_resources",
    Kind:           "new_resource",
    Group:          "category",
    Namespaced:     true,
    SupportedVerbs: AllVerbs,
},
```

1. The verb commands (get, create, delete, etc.) automatically work with any registered resource type.

### Adding a New Command

For resource-specific commands (beyond standard CRUD), see patterns in:

- `internal/cmd/lb.go` - load balancer specific commands
- `internal/cmd/security.go` - security resource commands
- `internal/cmd/cert.go` - certificate commands

Pattern: Create `newXxxCmd()` factory function returning `*cobra.Command`, add to `rootCmd` in `root.go`.

## Environment Variables

For integration testing:

- `F5XC_API_URL` - API base URL (required)
- `F5XC_API_TOKEN` - API token auth
- `F5XC_API_P12_FILE` + `F5XC_P12_PASSWORD` - P12 cert auth
- `F5XC_CERT_FILE` + `F5XC_KEY_FILE` - PEM cert auth

## Code Style

- Standard Go formatting (gofmt)
- Package doc comments required: `// Package name provides...`
- Error wrapping: `fmt.Errorf("context: %w", err)`
- YAML tags use kebab-case: `yaml:"field-name"`
- Linting via golangci-lint (see `.golangci.yml`)
