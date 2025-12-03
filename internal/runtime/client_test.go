package runtime

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/f5/f5xcctl/internal/auth"
)

// mockAuthenticator for testing.
type mockAuthenticator struct {
	token string
	err   error
}

func (m *mockAuthenticator) GetToken() (string, error) {
	return m.token, m.err
}

func (m *mockAuthenticator) GetHTTPClient() (*http.Client, error) {
	return &http.Client{Timeout: 30 * time.Second}, nil
}

// Ensure mockAuthenticator implements Authenticator.
var _ auth.Authenticator = (*mockAuthenticator)(nil)

// testClient creates a test client with a mock server.
func testClient(t *testing.T, handler http.HandlerFunc) (*Client, *httptest.Server) {
	server := httptest.NewServer(handler)

	client := &Client{
		httpClient:    &http.Client{Timeout: 10 * time.Second},
		baseURL:       server.URL,
		authenticator: &mockAuthenticator{token: "test-token"},
		debug:         false,
	}

	return client, server
}

func TestResponse_IsSuccess(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		want       bool
	}{
		{"200 OK", 200, true},
		{"201 Created", 201, true},
		{"204 No Content", 204, true},
		{"299", 299, true},
		{"300", 300, false},
		{"400 Bad Request", 400, false},
		{"401 Unauthorized", 401, false},
		{"404 Not Found", 404, false},
		{"500 Internal Server Error", 500, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp := &Response{StatusCode: tt.statusCode}
			assert.Equal(t, tt.want, resp.IsSuccess())
		})
	}
}

func TestResponse_DecodeJSON(t *testing.T) {
	type testStruct struct {
		Name  string `json:"name"`
		Value int    `json:"value"`
	}

	tests := []struct {
		name    string
		body    string
		want    testStruct
		wantErr bool
	}{
		{
			name: "valid JSON",
			body: `{"name":"test","value":42}`,
			want: testStruct{Name: "test", Value: 42},
		},
		{
			name:    "invalid JSON",
			body:    `{invalid}`,
			wantErr: true,
		},
		{
			name: "empty JSON object",
			body: `{}`,
			want: testStruct{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp := &Response{Body: []byte(tt.body)}
			var result testStruct
			err := resp.DecodeJSON(&result)

			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.want, result)
			}
		})
	}
}

func TestResponse_Error(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		body       string
		wantErr    bool
		wantMsg    string
	}{
		{
			name:       "success no error",
			statusCode: 200,
			body:       `{}`,
			wantErr:    false,
		},
		{
			name:       "error with message",
			statusCode: 400,
			body:       `{"code":"BAD_REQUEST","message":"Invalid input","details":"Missing required field"}`,
			wantErr:    true,
			wantMsg:    "400 Invalid input: Missing required field",
		},
		{
			name:       "error without JSON",
			statusCode: 500,
			body:       `Internal Server Error`,
			wantErr:    true,
			wantMsg:    "500 Internal Server Error",
		},
		{
			name:       "404 not found",
			statusCode: 404,
			body:       `{"message":"Resource not found"}`,
			wantErr:    true,
			wantMsg:    "404 Resource not found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp := &Response{
				StatusCode: tt.statusCode,
				Body:       []byte(tt.body),
			}
			err := resp.Error()

			if tt.wantErr {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantMsg)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestAPIError(t *testing.T) {
	err := &APIError{
		StatusCode: 404,
		Code:       "NOT_FOUND",
		Message:    "Resource not found",
		Details:    "Namespace 'test' does not exist",
	}

	assert.Equal(t, "404 Resource not found: Namespace 'test' does not exist", err.Error())
	assert.True(t, err.IsNotFound())
	assert.False(t, err.IsUnauthorized())
	assert.False(t, err.IsForbidden())
	assert.False(t, err.IsConflict())
}

func TestAPIError_StatusChecks(t *testing.T) {
	tests := []struct {
		name        string
		statusCode  int
		isNotFound  bool
		isUnauth    bool
		isForbidden bool
		isConflict  bool
	}{
		{"404", 404, true, false, false, false},
		{"401", 401, false, true, false, false},
		{"403", 403, false, false, true, false},
		{"409", 409, false, false, false, true},
		{"500", 500, false, false, false, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := &APIError{StatusCode: tt.statusCode, Message: "test"}
			assert.Equal(t, tt.isNotFound, err.IsNotFound())
			assert.Equal(t, tt.isUnauth, err.IsUnauthorized())
			assert.Equal(t, tt.isForbidden, err.IsForbidden())
			assert.Equal(t, tt.isConflict, err.IsConflict())
		})
	}
}

func TestClient_Get(t *testing.T) {
	client, server := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method)
		assert.Equal(t, "/api/test", r.URL.Path)
		assert.Equal(t, "value", r.URL.Query().Get("key"))
		assert.Equal(t, "APIToken test-token", r.Header.Get("Authorization"))

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{"result": "success"})
	})
	defer server.Close()

	query := url.Values{}
	query.Set("key", "value")

	resp, err := client.Get(context.Background(), "/api/test", query)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Contains(t, string(resp.Body), "success")
}

func TestClient_Post(t *testing.T) {
	client, server := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "/api/resources", r.URL.Path)
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))

		var body map[string]string
		json.NewDecoder(r.Body).Decode(&body)
		assert.Equal(t, "test-name", body["name"])

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(map[string]string{"id": "123"})
	})
	defer server.Close()

	resp, err := client.Post(context.Background(), "/api/resources", map[string]string{"name": "test-name"})
	require.NoError(t, err)
	assert.Equal(t, http.StatusCreated, resp.StatusCode)
}

func TestClient_Put(t *testing.T) {
	client, server := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPut, r.Method)
		assert.Equal(t, "/api/resources/123", r.URL.Path)

		w.WriteHeader(http.StatusOK)
	})
	defer server.Close()

	resp, err := client.Put(context.Background(), "/api/resources/123", map[string]string{"name": "updated"})
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

func TestClient_Delete(t *testing.T) {
	client, server := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodDelete, r.Method)
		assert.Equal(t, "/api/resources/123", r.URL.Path)

		w.WriteHeader(http.StatusNoContent)
	})
	defer server.Close()

	resp, err := client.Delete(context.Background(), "/api/resources/123")
	require.NoError(t, err)
	assert.Equal(t, http.StatusNoContent, resp.StatusCode)
}

func TestClient_Do_WithHeaders(t *testing.T) {
	client, server := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "custom-value", r.Header.Get("X-Custom-Header"))
		w.WriteHeader(http.StatusOK)
	})
	defer server.Close()

	req := &Request{
		Method: http.MethodGet,
		Path:   "/api/test",
		Headers: map[string]string{
			"X-Custom-Header": "custom-value",
		},
	}

	resp, err := client.Do(context.Background(), req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

func TestClient_Do_ContextCancellation(t *testing.T) {
	client, server := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(100 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	})
	defer server.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	_, err := client.Get(ctx, "/api/test", nil)
	assert.Error(t, err)
}

func TestClient_Do_ServerError(t *testing.T) {
	client, server := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{
			"message": "Internal error",
			"details": "Database connection failed",
		})
	})
	defer server.Close()

	resp, err := client.Get(context.Background(), "/api/test", nil)
	require.NoError(t, err) // No transport error
	assert.Equal(t, http.StatusInternalServerError, resp.StatusCode)

	apiErr := resp.Error()
	assert.Error(t, apiErr)

	var ae *APIError
	assert.ErrorAs(t, apiErr, &ae)
	assert.Equal(t, 500, ae.StatusCode)
}

func TestWithDebug(t *testing.T) {
	client := &Client{}
	opt := WithDebug(true)
	opt(client)
	assert.True(t, client.debug)
}

func TestRequest_Struct(t *testing.T) {
	req := Request{
		Method:      http.MethodPost,
		Path:        "/api/test",
		Query:       url.Values{"key": []string{"value"}},
		Body:        map[string]string{"name": "test"},
		Headers:     map[string]string{"X-Custom": "value"},
		ContentType: "application/json",
	}

	assert.Equal(t, http.MethodPost, req.Method)
	assert.Equal(t, "/api/test", req.Path)
	assert.Equal(t, "value", req.Query.Get("key"))
	assert.NotNil(t, req.Body)
	assert.Equal(t, "value", req.Headers["X-Custom"])
}

func TestResponse_Struct(t *testing.T) {
	headers := http.Header{}
	headers.Set("Content-Type", "application/json")

	resp := Response{
		StatusCode: 200,
		Headers:    headers,
		Body:       []byte(`{"test":true}`),
	}

	assert.Equal(t, 200, resp.StatusCode)
	assert.Equal(t, "application/json", resp.Headers.Get("Content-Type"))
	assert.Equal(t, `{"test":true}`, string(resp.Body))
}
