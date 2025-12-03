package integration

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/f5/f5xcctl/internal/runtime"
)

// NamespaceSpec represents a namespace specification.
type NamespaceSpec struct {
	// GCSpec is the garbage collection specification
	GCSpec interface{} `json:"gc_spec,omitempty"`
}

// NamespaceMetadata represents namespace metadata.
type NamespaceMetadata struct {
	Name        string            `json:"name"`
	Namespace   string            `json:"namespace,omitempty"`
	Labels      map[string]string `json:"labels,omitempty"`
	Annotations map[string]string `json:"annotations,omitempty"`
	Description string            `json:"description,omitempty"`
	UID         string            `json:"uid,omitempty"`
}

// Namespace represents an F5XC namespace.
type Namespace struct {
	Metadata   NamespaceMetadata `json:"metadata"`
	Spec       NamespaceSpec     `json:"spec,omitempty"`
	SystemMeta interface{}       `json:"system_metadata,omitempty"`
}

// NamespaceListResponse represents the response from listing namespaces.
type NamespaceListResponse struct {
	Items []Namespace `json:"items"`
}

// TestNamespaceList tests listing namespaces.
func TestNamespaceList(t *testing.T) {
	client := getClient(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// List namespaces
	resp, err := client.Get(ctx, "/api/web/namespaces", nil)
	require.NoError(t, err, "should list namespaces without error")
	require.NotNil(t, resp)

	t.Logf("Response status: %d", resp.StatusCode)

	if !resp.IsSuccess() {
		t.Logf("Response body: %s", string(resp.Body))
	}

	assert.Equal(t, http.StatusOK, resp.StatusCode, "should return 200 OK")

	// Parse response
	var listResp NamespaceListResponse
	err = resp.DecodeJSON(&listResp)
	require.NoError(t, err, "should decode namespace list response")

	t.Logf("Found %d namespaces", len(listResp.Items))

	// Log namespace names
	for _, ns := range listResp.Items {
		t.Logf("  - %s", ns.Metadata.Name)
	}

	// Should have at least the system namespaces
	assert.NotEmpty(t, listResp.Items, "should have at least one namespace")
}

// TestNamespaceGet tests getting a specific namespace (system namespace).
func TestNamespaceGet(t *testing.T) {
	client := getClient(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Get the "system" namespace which should always exist
	resp, err := client.Get(ctx, "/api/web/namespaces/system", nil)
	require.NoError(t, err, "should get namespace without error")
	require.NotNil(t, resp)

	t.Logf("Response status: %d", resp.StatusCode)

	if !resp.IsSuccess() {
		t.Logf("Response body: %s", string(resp.Body))
	}

	assert.Equal(t, http.StatusOK, resp.StatusCode, "should return 200 OK")

	// Parse response
	var ns Namespace
	err = resp.DecodeJSON(&ns)
	require.NoError(t, err, "should decode namespace response")

	assert.Equal(t, "system", ns.Metadata.Name, "should return system namespace")
	t.Logf("Got namespace: %s", ns.Metadata.Name)
}

// TestNamespaceCRUD tests create, read, update, delete operations for namespaces.
func TestNamespaceCRUD(t *testing.T) {
	client := getClient(t)

	// Generate unique namespace name for this test
	nsName := "test-integration-" + time.Now().Format("20060102-150405")
	t.Logf("Testing with namespace: %s", nsName)

	// Cleanup function to ensure namespace is deleted
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		// Attempt cleanup, ignore errors
		client.Post(ctx, "/api/web/namespaces/"+nsName+"/cascade_delete", map[string]interface{}{})
	}()

	// Test Create
	t.Run("Create", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		createReq := map[string]interface{}{
			"metadata": map[string]interface{}{
				"name": nsName,
				"labels": map[string]string{
					"test":       "integration",
					"created-by": "f5xc-cli",
				},
				"description": "Integration test namespace",
			},
			"spec": map[string]interface{}{},
		}

		resp, err := client.Post(ctx, "/api/web/namespaces", createReq)
		require.NoError(t, err, "should create namespace without error")
		require.NotNil(t, resp)

		t.Logf("Create response status: %d", resp.StatusCode)
		if !resp.IsSuccess() {
			t.Logf("Create response body: %s", string(resp.Body))
		}

		// Accept 200, 201, or 409 (already exists from previous failed test)
		assert.True(t, resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusCreated || resp.StatusCode == http.StatusConflict,
			"should return 200, 201, or 409, got %d", resp.StatusCode)

		if resp.IsSuccess() {
			var ns Namespace
			err = resp.DecodeJSON(&ns)
			require.NoError(t, err, "should decode created namespace")
			assert.Equal(t, nsName, ns.Metadata.Name)
		}
	})

	// Test Read
	t.Run("Read", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		// Wait a moment for namespace to be ready
		time.Sleep(2 * time.Second)

		resp, err := client.Get(ctx, "/api/web/namespaces/"+nsName, nil)
		require.NoError(t, err, "should get namespace without error")
		require.NotNil(t, resp)

		t.Logf("Read response status: %d", resp.StatusCode)
		if !resp.IsSuccess() {
			t.Logf("Read response body: %s", string(resp.Body))
		}

		assert.Equal(t, http.StatusOK, resp.StatusCode, "should return 200 OK")

		var ns Namespace
		err = resp.DecodeJSON(&ns)
		require.NoError(t, err, "should decode namespace")
		assert.Equal(t, nsName, ns.Metadata.Name)
		assert.Equal(t, "integration", ns.Metadata.Labels["test"])
	})

	// Test Update (using replace)
	t.Run("Update", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		updateReq := map[string]interface{}{
			"metadata": map[string]interface{}{
				"name": nsName,
				"labels": map[string]string{
					"test":       "integration",
					"created-by": "f5xc-cli",
					"updated":    "true",
				},
				"description": "Updated integration test namespace",
			},
			"spec": map[string]interface{}{},
		}

		resp, err := client.Put(ctx, "/api/web/namespaces/"+nsName, updateReq)
		require.NoError(t, err, "should update namespace without error")
		require.NotNil(t, resp)

		t.Logf("Update response status: %d", resp.StatusCode)
		if !resp.IsSuccess() {
			t.Logf("Update response body: %s", string(resp.Body))
		}

		assert.Equal(t, http.StatusOK, resp.StatusCode, "should return 200 OK")

		// Verify update
		resp, err = client.Get(ctx, "/api/web/namespaces/"+nsName, nil)
		require.NoError(t, err)
		require.True(t, resp.IsSuccess())

		var ns Namespace
		err = resp.DecodeJSON(&ns)
		require.NoError(t, err)
		assert.Equal(t, "true", ns.Metadata.Labels["updated"])
	})

	// Test Delete
	t.Run("Delete", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		// Use cascade_delete to ensure cleanup
		resp, err := client.Post(ctx, "/api/web/namespaces/"+nsName+"/cascade_delete", map[string]interface{}{})
		require.NoError(t, err, "should delete namespace without error")
		require.NotNil(t, resp)

		t.Logf("Delete response status: %d", resp.StatusCode)
		if !resp.IsSuccess() {
			t.Logf("Delete response body: %s", string(resp.Body))
		}

		// Accept 200, 202 (accepted), or 404 (already deleted)
		assert.True(t, resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusAccepted || resp.StatusCode == http.StatusNotFound,
			"should return 200, 202, or 404, got %d", resp.StatusCode)

		// Wait for deletion
		time.Sleep(2 * time.Second)

		// Verify deletion
		resp, err = client.Get(ctx, "/api/web/namespaces/"+nsName, nil)
		require.NoError(t, err)
		assert.Equal(t, http.StatusNotFound, resp.StatusCode, "namespace should be deleted")
	})
}

// TestAPIConnection tests basic API connectivity.
func TestAPIConnection(t *testing.T) {
	client := getClient(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Simple health check - list namespaces
	resp, err := client.Get(ctx, "/api/web/namespaces", nil)
	require.NoError(t, err, "should connect to API without error")
	require.NotNil(t, resp)

	t.Logf("API connection test - Status: %d", resp.StatusCode)

	// We should not get an authentication error
	assert.NotEqual(t, http.StatusUnauthorized, resp.StatusCode, "should not be unauthorized")
	assert.NotEqual(t, http.StatusForbidden, resp.StatusCode, "should not be forbidden")

	if resp.IsSuccess() {
		t.Log("API connection successful - credentials are valid")
	} else {
		t.Logf("API response body: %s", string(resp.Body))
	}
}

// TestDebugMode tests that debug mode outputs request/response info.
func TestDebugMode(t *testing.T) {
	// Create a debug-enabled client
	debugClient, err := runtime.NewClientFromEnv(runtime.WithDebug(true))
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// This should produce debug output
	resp, err := debugClient.Get(ctx, "/api/web/namespaces", nil)
	require.NoError(t, err)
	require.NotNil(t, resp)

	// We just verify no errors - actual debug output goes to stdout
	t.Logf("Debug request completed with status: %d", resp.StatusCode)
}
