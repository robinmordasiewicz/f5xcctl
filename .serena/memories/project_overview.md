# f5xcctl - F5 Distributed Cloud CLI

## Purpose
A kubectl-style command-line interface for managing F5 Distributed Cloud (F5XC) resources. Provides CRUD operations for load balancers, origin pools, firewalls, certificates, DNS, and more.

## Tech Stack
- **Language**: Go 1.25.3
- **CLI Framework**: Cobra (github.com/spf13/cobra)
- **Config Management**: Viper (github.com/spf13/viper)
- **HTTP Client**: go-retryablehttp (with automatic retries)
- **Testing**: stretchr/testify
- **Config Format**: YAML (gopkg.in/yaml.v3)
- **Certificate Handling**: go-pkcs12 for P12 bundles

## Authentication Methods
1. **API Token** - Bearer token authentication
2. **Certificate** - X.509 client certificate (cert + key files)
3. **P12/PKCS#12** - Certificate bundle with password
4. **Browser/SSO** - OAuth flow with browser callback

## Project Structure
```
f5xcctl/
├── cmd/f5xcctl/          # Entry point (main.go)
├── internal/
│   ├── cmd/              # CLI commands (root, verbs, resources)
│   ├── config/           # Configuration management
│   ├── auth/             # Authentication (token, cert, p12, sso)
│   ├── runtime/          # API client
│   └── output/           # Output formatters (table, json, yaml)
├── integration/          # Integration tests
├── pkg/jmespath/         # JMESPath query support
├── scripts/              # Build/generation scripts
├── docs/                 # Documentation
└── templates/            # YAML templates
```

## Key Commands
- `f5xcctl` - Interactive mode (default)
- `f5xcctl get <resource>` - List resources
- `f5xcctl describe <resource> <name>` - Show details
- `f5xcctl apply -f <file>` - Create/update from YAML
- `f5xcctl delete <resource> <name>` - Delete resource
- `f5xcctl configure` - Setup configuration

## Configuration
- Config file: `~/.f5xc/config.yaml`
- Credentials: `~/.f5xc/credentials` (restricted permissions)
- Supports multiple profiles

## Environment Variables
- `F5XC_API_TOKEN` - API token
- `F5XC_API_URL` - API endpoint URL
- `F5XC_TENANT` - Tenant name
- `F5XC_NAMESPACE` - Default namespace
- `F5XC_API_P12_FILE` - P12 certificate file
- `F5XC_P12_PASSWORD` - P12 password
