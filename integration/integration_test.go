// Package integration provides integration tests for the f5xcctl CLI.
// These tests require real API credentials and interact with a live F5XC environment.
//
// To run integration tests:
//
//	export F5XC_API_URL="https://your-tenant.console.ves.volterra.io"
//	export F5XC_API_P12_FILE="/path/to/your.p12"
//	export F5XC_P12_PASSWORD="your-password"
//	go test -v ./integration/... -tags=integration
//
// Or with API token:
//
//	export F5XC_API_URL="https://your-tenant.console.ves.volterra.io"
//	export F5XC_API_TOKEN="your-api-token"
//	go test -v ./integration/... -tags=integration
package integration

import (
	"os"
	"testing"

	"github.com/f5/f5xcctl/internal/runtime"
)

// testClient is a shared client for integration tests.
var testClient *runtime.Client

// TestMain sets up the integration test environment.
func TestMain(m *testing.M) {
	// Check if integration tests should run
	if os.Getenv("F5XC_API_URL") == "" {
		// Skip integration tests if no API URL is configured
		os.Exit(0)
	}

	// Create test client
	var err error
	testClient, err = runtime.NewClientFromEnv(runtime.WithDebug(os.Getenv("F5XC_DEBUG") == "true"))
	if err != nil {
		panic("failed to create test client: " + err.Error())
	}

	// Run tests
	os.Exit(m.Run())
}

// skipIfNoClient skips the test if the test client is not available.
func skipIfNoClient(t *testing.T) {
	t.Helper()
	if testClient == nil {
		t.Skip("integration test client not available")
	}
}

// getClient returns the shared test client.
func getClient(t *testing.T) *runtime.Client {
	t.Helper()
	skipIfNoClient(t)
	return testClient
}
