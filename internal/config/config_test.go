package config

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// setTestHome sets the home directory environment variable for testing.
// On Windows, this sets USERPROFILE; on Unix systems, this sets HOME.
// Returns a cleanup function to restore the original values.
func setTestHome(t *testing.T, dir string) func() {
	t.Helper()

	if runtime.GOOS == "windows" {
		original := os.Getenv("USERPROFILE")
		os.Setenv("USERPROFILE", dir)
		return func() {
			os.Setenv("USERPROFILE", original)
		}
	}

	original := os.Getenv("HOME")
	os.Setenv("HOME", dir)
	return func() {
		os.Setenv("HOME", original)
	}
}

func TestNewDefault(t *testing.T) {
	cfg := NewDefault()

	assert.Equal(t, "default", cfg.CurrentProfile)
	assert.Contains(t, cfg.Profiles, "default")
	assert.Equal(t, "api-token", cfg.Profiles["default"].AuthMethod)
	assert.Equal(t, "default", cfg.Profiles["default"].DefaultNamespace)
	assert.Equal(t, "table", cfg.Profiles["default"].OutputFormat)
}

func TestGetCurrentProfile(t *testing.T) {
	tests := []struct {
		name           string
		config         *Config
		expectedNil    bool
		expectedTenant string
	}{
		{
			name:        "nil config",
			config:      nil,
			expectedNil: true,
		},
		{
			name: "valid profile",
			config: &Config{
				CurrentProfile: "production",
				Profiles: map[string]Profile{
					"production": {Tenant: "prod-tenant"},
				},
			},
			expectedNil:    false,
			expectedTenant: "prod-tenant",
		},
		{
			name: "missing profile",
			config: &Config{
				CurrentProfile: "nonexistent",
				Profiles:       map[string]Profile{},
			},
			expectedNil: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			profile := tt.config.GetCurrentProfile()
			if tt.expectedNil {
				assert.Nil(t, profile)
			} else {
				require.NotNil(t, profile)
				assert.Equal(t, tt.expectedTenant, profile.Tenant)
			}
		})
	}
}

func TestConfigGetSet(t *testing.T) {
	cfg := NewDefault()

	// Test Get
	value, err := cfg.Get("auth-method")
	assert.NoError(t, err)
	assert.Equal(t, "api-token", value)

	// Test Set
	err = cfg.Set("tenant", "my-tenant")
	assert.NoError(t, err)

	value, err = cfg.Get("tenant")
	assert.NoError(t, err)
	assert.Equal(t, "my-tenant", value)

	// Test unknown key
	_, err = cfg.Get("unknown-key")
	assert.Error(t, err)

	err = cfg.Set("unknown-key", "value")
	assert.Error(t, err)
}

func TestSaveAndLoad(t *testing.T) {
	// Create temp directory
	tmpDir, err := os.MkdirTemp("", "f5xc-test")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	// Override default config dir (handles Windows vs Unix)
	cleanup := setTestHome(t, tmpDir)
	defer cleanup()

	// Create and save config
	cfg := &Config{
		CurrentProfile: "test",
		Profiles: map[string]Profile{
			"test": {
				Tenant:           "test-tenant",
				APIURL:           "https://test.api.com",
				AuthMethod:       "api-token",
				DefaultNamespace: "test-ns",
				OutputFormat:     "json",
			},
		},
	}

	err = Save(cfg)
	require.NoError(t, err)

	// Verify file exists
	configPath := filepath.Join(tmpDir, ".f5xc", "config.yaml")
	_, err = os.Stat(configPath)
	assert.NoError(t, err)

	// Load config
	loaded, err := Load("", "")
	require.NoError(t, err)

	assert.Equal(t, cfg.CurrentProfile, loaded.CurrentProfile)
	assert.Equal(t, cfg.Profiles["test"].Tenant, loaded.Profiles["test"].Tenant)
}

func TestLoadWithProfile(t *testing.T) {
	// Create temp directory
	tmpDir, err := os.MkdirTemp("", "f5xc-test")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	// Override default config dir (handles Windows vs Unix)
	cleanup := setTestHome(t, tmpDir)
	defer cleanup()

	// Create config with multiple profiles
	cfg := &Config{
		CurrentProfile: "default",
		Profiles: map[string]Profile{
			"default": {Tenant: "default-tenant"},
			"prod":    {Tenant: "prod-tenant"},
		},
	}

	err = Save(cfg)
	require.NoError(t, err)

	// Load with specific profile
	loaded, err := Load("", "prod")
	require.NoError(t, err)
	assert.Equal(t, "prod", loaded.CurrentProfile)

	// Load with nonexistent profile
	_, err = Load("", "nonexistent")
	assert.Error(t, err)
}

func TestCredentials(t *testing.T) {
	// Create temp directory
	tmpDir, err := os.MkdirTemp("", "f5xc-test")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	// Override default config dir (handles Windows vs Unix)
	cleanup := setTestHome(t, tmpDir)
	defer cleanup()

	// Create and save credentials
	creds := &Credentials{
		Profiles: map[string]ProfileCredentials{
			"default": {APIToken: "test-token"},
		},
	}

	err = SaveCredentials(creds)
	require.NoError(t, err)

	// Verify file permissions (skip on Windows as it doesn't support Unix permissions)
	credsPath := filepath.Join(tmpDir, ".f5xc", "credentials")
	info, err := os.Stat(credsPath)
	require.NoError(t, err)
	if runtime.GOOS != "windows" {
		assert.Equal(t, os.FileMode(0600), info.Mode().Perm())
	}

	// Load credentials
	loaded, err := LoadCredentials()
	require.NoError(t, err)
	assert.Equal(t, "test-token", loaded.Profiles["default"].APIToken)
}
