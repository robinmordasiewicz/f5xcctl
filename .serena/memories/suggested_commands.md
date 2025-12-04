# Development Commands

## Build

```bash
make build              # Build binary (./f5xcctl)
make build-all          # Build for all platforms
make install            # Install to GOPATH/bin
```

## Testing

```bash
make test               # Run all tests with coverage
make test-unit          # Run unit tests only
make test-integration   # Run integration tests (requires env vars)
make test-short         # Run short tests
make coverage           # Generate HTML coverage report
```

## Code Quality

```bash
make lint               # Run golangci-lint
make fmt                # Format code (gofmt)
make vet                # Run go vet
make check              # Run all checks (fmt, vet, lint, test)
```

## Dependencies

```bash
make deps               # Download and tidy dependencies
go mod tidy             # Tidy go.mod
```

## Running

```bash
make run                # Build and run
make run ARGS="get ns"  # Build and run with arguments
./f5xcctl               # Run directly (interactive mode)
./f5xcctl get namespace # Run with command
```

## Cleanup

```bash
make clean              # Remove build artifacts
```

## Release

```bash
make release            # Build release with goreleaser
make release-snapshot   # Build snapshot (no publish)
```

## Documentation

```bash
make docs               # Generate CLI documentation
make completion         # Generate shell completions
```

## Integration Test Requirements

Set these environment variables:

- `F5XC_API_URL` - API base URL (required)
- `F5XC_API_P12_FILE` - Path to P12 certificate file
- `F5XC_P12_PASSWORD` - P12 password
- OR `F5XC_API_TOKEN` - API token

## Linter (golangci-lint)

Enabled linters:

- govet, ineffassign, staticcheck, unused, misspell, gocritic

Formatters:

- gofmt, goimports
