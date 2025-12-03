# Code Style and Conventions

## Go Standards
- Go 1.25.3 or later
- Standard Go formatting (gofmt, goimports)
- Follow Effective Go guidelines

## Package Comments
Every package must have a package-level doc comment:
```go
// Package cmd provides the CLI commands for f5xcctl.
package cmd
```

## Naming Conventions
- **Exported functions**: PascalCase (`GetNamespace`, `SetVersionInfo`)
- **Private functions**: camelCase (`initConfig`, `loadCertificate`)
- **Constants**: PascalCase for exported, camelCase for private
- **Acronyms**: Treat as single word (`apiURL`, `APIURL`)

## Error Handling
- Use `fmt.Errorf("context: %w", err)` for wrapping errors
- Return descriptive error messages with context
- Check errors immediately after function calls

## Struct Tags
- YAML: `yaml:"field-name,omitempty"`
- JSON: `json:"field_name,omitempty"`
- Use kebab-case for YAML, snake_case for JSON

## Function Comments
Document exported functions:
```go
// GetNamespace returns the namespace to use for operations
func GetNamespace() string {
```

## File Organization
- Group imports: stdlib, external deps, internal packages
- Put related functions together
- Types before functions that use them

## CLI Patterns
- Use Cobra for commands
- Use Viper for configuration binding
- Global flags defined in `root.go`
- Subcommands in separate files

## Testing
- Use testify/assert for assertions
- Use testify/require for fatal assertions
- Table-driven tests preferred
- Test files: `*_test.go` alongside source

## Linter Rules
See `.golangci.yml`:
- Disabled: errcheck (many false positives for CLI)
- Disabled: gosec (false positives for file operations)
- Enabled: gocritic (diagnostic, performance tags)
