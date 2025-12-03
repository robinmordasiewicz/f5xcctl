// Package config provides configuration management for the f5xcctl CLI.
package config

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"gopkg.in/yaml.v3"
)

// Config represents the CLI configuration.
type Config struct {
	CurrentProfile string             `yaml:"current-profile"`
	Profiles       map[string]Profile `yaml:"profiles"`
}

// Profile represents a configuration profile.
type Profile struct {
	Tenant           string `yaml:"tenant"`
	APIURL           string `yaml:"api-url"`
	AuthMethod       string `yaml:"auth-method"` // "api-token", "certificate", "p12", "sso"
	CertFile         string `yaml:"cert-file,omitempty"`
	KeyFile          string `yaml:"key-file,omitempty"`
	P12File          string `yaml:"p12-file,omitempty"`
	DefaultNamespace string `yaml:"default-namespace"`
	OutputFormat     string `yaml:"output-format"`
}

// Credentials represents stored credentials (separate file with restricted permissions).
type Credentials struct {
	Profiles map[string]ProfileCredentials `yaml:"profiles"`
}

// ProfileCredentials represents credentials for a profile.
type ProfileCredentials struct {
	APIToken    string    `yaml:"api-token,omitempty"`
	P12Password string    `yaml:"p12-password,omitempty"`
	ExpiresAt   time.Time `yaml:"expires-at,omitempty"`
}

// DefaultConfigDir returns the default configuration directory.
func DefaultConfigDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ".f5xc"
	}
	return filepath.Join(home, ".f5xc")
}

// DefaultConfigPath returns the default configuration file path.
func DefaultConfigPath() string {
	return filepath.Join(DefaultConfigDir(), "config.yaml")
}

// DefaultCredentialsPath returns the default credentials file path.
func DefaultCredentialsPath() string {
	return filepath.Join(DefaultConfigDir(), "credentials")
}

// NewDefault creates a new default configuration.
func NewDefault() *Config {
	return &Config{
		CurrentProfile: "default",
		Profiles: map[string]Profile{
			"default": {
				AuthMethod:       "api-token",
				DefaultNamespace: "default",
				OutputFormat:     "table",
			},
		},
	}
}

// Load loads the configuration from file.
func Load(configFile, profileName string) (*Config, error) {
	if configFile == "" {
		configFile = DefaultConfigPath()
	}

	data, err := os.ReadFile(configFile)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("configuration file not found: %s (run 'f5xcctl configure')", configFile)
		}
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	// Override current profile if specified
	if profileName != "" {
		if _, ok := cfg.Profiles[profileName]; !ok {
			return nil, fmt.Errorf("profile %q not found in configuration", profileName)
		}
		cfg.CurrentProfile = profileName
	}

	return &cfg, nil
}

// Save saves the configuration to file.
func Save(cfg *Config) error {
	configDir := DefaultConfigDir()
	if err := os.MkdirAll(configDir, 0700); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	configPath := DefaultConfigPath()
	if err := os.WriteFile(configPath, data, 0600); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	return nil
}

// LoadCredentials loads credentials from file.
func LoadCredentials() (*Credentials, error) {
	credsPath := DefaultCredentialsPath()

	data, err := os.ReadFile(credsPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("credentials file not found")
		}
		return nil, fmt.Errorf("failed to read credentials file: %w", err)
	}

	var creds Credentials
	if err := yaml.Unmarshal(data, &creds); err != nil {
		return nil, fmt.Errorf("failed to parse credentials file: %w", err)
	}

	return &creds, nil
}

// SaveCredentials saves credentials to file with restricted permissions.
func SaveCredentials(creds *Credentials) error {
	configDir := DefaultConfigDir()
	if err := os.MkdirAll(configDir, 0700); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	data, err := yaml.Marshal(creds)
	if err != nil {
		return fmt.Errorf("failed to marshal credentials: %w", err)
	}

	credsPath := DefaultCredentialsPath()
	if err := os.WriteFile(credsPath, data, 0600); err != nil {
		return fmt.Errorf("failed to write credentials file: %w", err)
	}

	return nil
}

// GetCurrentProfile returns the current profile configuration.
func (c *Config) GetCurrentProfile() *Profile {
	if c == nil {
		return nil
	}
	profile, ok := c.Profiles[c.CurrentProfile]
	if !ok {
		return nil
	}
	return &profile
}

// Get retrieves a configuration value by key.
func (c *Config) Get(key string) (string, error) {
	profile := c.GetCurrentProfile()
	if profile == nil {
		return "", fmt.Errorf("no profile configured")
	}

	switch key {
	case "tenant":
		return profile.Tenant, nil
	case "api-url":
		return profile.APIURL, nil
	case "auth-method":
		return profile.AuthMethod, nil
	case "default-namespace":
		return profile.DefaultNamespace, nil
	case "output-format":
		return profile.OutputFormat, nil
	case "cert-file":
		return profile.CertFile, nil
	case "key-file":
		return profile.KeyFile, nil
	case "p12-file":
		return profile.P12File, nil
	case "current-profile":
		return c.CurrentProfile, nil
	default:
		return "", fmt.Errorf("unknown configuration key: %s", key)
	}
}

// Set sets a configuration value by key.
func (c *Config) Set(key, value string) error {
	profile := c.GetCurrentProfile()
	if profile == nil {
		return fmt.Errorf("no profile configured")
	}

	switch key {
	case "tenant":
		profile.Tenant = value
	case "api-url":
		profile.APIURL = value
	case "auth-method":
		profile.AuthMethod = value
	case "default-namespace":
		profile.DefaultNamespace = value
	case "output-format":
		profile.OutputFormat = value
	case "cert-file":
		profile.CertFile = value
	case "key-file":
		profile.KeyFile = value
	case "p12-file":
		profile.P12File = value
	case "current-profile":
		if _, ok := c.Profiles[value]; !ok {
			return fmt.Errorf("profile %q does not exist", value)
		}
		c.CurrentProfile = value
		return nil
	default:
		return fmt.Errorf("unknown configuration key: %s", key)
	}

	c.Profiles[c.CurrentProfile] = *profile
	return nil
}
