// Package runtime provides the HTTP client and request execution for the f5xcctl CLI.
package runtime

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/hashicorp/go-retryablehttp"

	"github.com/f5/f5xcctl/internal/auth"
	"github.com/f5/f5xcctl/internal/config"
)

// Client is the F5XC API client.
type Client struct {
	httpClient    *http.Client
	baseURL       string
	authenticator auth.Authenticator
	debug         bool
}

// ClientOption is a function that configures the client.
type ClientOption func(*Client)

// WithDebug enables debug output.
func WithDebug(debug bool) ClientOption {
	return func(c *Client) {
		c.debug = debug
	}
}

// NewClient creates a new API client.
func NewClient(cfg *config.Config, creds *config.Credentials) (*Client, error) {
	profile := cfg.GetCurrentProfile()
	if profile == nil {
		return nil, fmt.Errorf("no profile configured")
	}

	var authenticator auth.Authenticator

	// Determine authentication method
	switch profile.AuthMethod {
	case "certificate":
		if profile.CertFile == "" || profile.KeyFile == "" {
			return nil, fmt.Errorf("certificate and key files required for certificate authentication")
		}
		authenticator = auth.NewCertAuth(profile.CertFile, profile.KeyFile)
	case "p12":
		if profile.P12File == "" {
			return nil, fmt.Errorf("P12 file required for P12 certificate authentication")
		}
		// P12 password is retrieved from credentials
		var p12Password string
		if creds != nil {
			if profileCreds, ok := creds.Profiles[cfg.CurrentProfile]; ok {
				p12Password = profileCreds.P12Password
			}
		}
		authenticator = auth.NewP12Auth(profile.P12File, p12Password)
	default:
		if creds == nil {
			return nil, fmt.Errorf("credentials required for API token authentication")
		}
		profileCreds, ok := creds.Profiles[cfg.CurrentProfile]
		if !ok || profileCreds.APIToken == "" {
			return nil, fmt.Errorf("no API token found for profile %q", cfg.CurrentProfile)
		}
		authenticator = auth.NewTokenAuth(profileCreds.APIToken)
	}

	// Create retryable HTTP client
	retryClient := retryablehttp.NewClient()
	retryClient.RetryMax = 3
	retryClient.RetryWaitMin = 1 * time.Second
	retryClient.RetryWaitMax = 30 * time.Second
	retryClient.Logger = nil // Disable default logging

	// Custom retry policy
	retryClient.CheckRetry = func(ctx context.Context, resp *http.Response, err error) (bool, error) {
		// Don't retry on context cancellation
		if ctx.Err() != nil {
			return false, ctx.Err()
		}

		// Retry on connection errors
		if err != nil {
			return true, err
		}

		// Retry on specific status codes
		switch resp.StatusCode {
		case http.StatusTooManyRequests, // 429
			http.StatusServiceUnavailable, // 503
			http.StatusGatewayTimeout:     // 504
			return true, nil
		}

		return false, nil
	}

	// Get HTTP client from authenticator (for cert auth)
	httpClient, err := authenticator.GetHTTPClient()
	if err != nil {
		return nil, err
	}

	// Use the authenticator's transport if it has one
	if httpClient.Transport != nil {
		retryClient.HTTPClient.Transport = httpClient.Transport
	}

	return &Client{
		httpClient:    retryClient.StandardClient(),
		baseURL:       strings.TrimSuffix(profile.APIURL, "/"),
		authenticator: authenticator,
	}, nil
}

// NewClientFromEnv creates a new API client from environment variables.
// This is primarily used for integration testing.
// Supported environment variables:
//   - F5XC_API_URL: The API base URL (required)
//   - F5XC_API_TOKEN: API token for token-based auth
//   - F5XC_API_P12_FILE: Path to P12 certificate file
//   - F5XC_P12_PASSWORD: Password for P12 file
//   - F5XC_CERT_FILE: Path to certificate file (PEM)
//   - F5XC_KEY_FILE: Path to key file (PEM)
func NewClientFromEnv(opts ...ClientOption) (*Client, error) {
	apiURL := os.Getenv("F5XC_API_URL")
	if apiURL == "" {
		return nil, fmt.Errorf("F5XC_API_URL environment variable is required")
	}

	var authenticator auth.Authenticator

	// Check for P12 auth first
	p12File := os.Getenv("F5XC_API_P12_FILE")
	p12Password := os.Getenv("F5XC_P12_PASSWORD")

	// Check for certificate auth
	certFile := os.Getenv("F5XC_CERT_FILE")
	keyFile := os.Getenv("F5XC_KEY_FILE")

	// Check for token auth
	apiToken := os.Getenv("F5XC_API_TOKEN")

	switch {
	case p12File != "":
		authenticator = auth.NewP12Auth(p12File, p12Password)
	case certFile != "" && keyFile != "":
		authenticator = auth.NewCertAuth(certFile, keyFile)
	case apiToken != "":
		authenticator = auth.NewTokenAuth(apiToken)
	default:
		return nil, fmt.Errorf("no authentication method configured: set F5XC_API_TOKEN, F5XC_API_P12_FILE, or F5XC_CERT_FILE/F5XC_KEY_FILE")
	}

	// Create retryable HTTP client
	retryClient := retryablehttp.NewClient()
	retryClient.RetryMax = 3
	retryClient.RetryWaitMin = 1 * time.Second
	retryClient.RetryWaitMax = 30 * time.Second
	retryClient.Logger = nil

	// Custom retry policy
	retryClient.CheckRetry = func(ctx context.Context, resp *http.Response, err error) (bool, error) {
		if ctx.Err() != nil {
			return false, ctx.Err()
		}
		if err != nil {
			return true, err
		}
		switch resp.StatusCode {
		case http.StatusTooManyRequests, http.StatusServiceUnavailable, http.StatusGatewayTimeout:
			return true, nil
		}
		return false, nil
	}

	// Get HTTP client from authenticator (for cert auth)
	httpClient, err := authenticator.GetHTTPClient()
	if err != nil {
		return nil, err
	}

	// Use the authenticator's transport if it has one
	if httpClient.Transport != nil {
		retryClient.HTTPClient.Transport = httpClient.Transport
	}

	client := &Client{
		httpClient:    retryClient.StandardClient(),
		baseURL:       strings.TrimSuffix(apiURL, "/"),
		authenticator: authenticator,
	}

	// Apply options
	for _, opt := range opts {
		opt(client)
	}

	return client, nil
}

// Request represents an API request.
type Request struct {
	Method      string
	Path        string
	Query       url.Values
	Body        interface{}
	Headers     map[string]string
	ContentType string
}

// Response represents an API response.
type Response struct {
	StatusCode int
	Headers    http.Header
	Body       []byte
}

// Do executes an API request.
func (c *Client) Do(ctx context.Context, req *Request) (*Response, error) {
	// Build URL
	reqURL := c.baseURL + req.Path
	if len(req.Query) > 0 {
		reqURL += "?" + req.Query.Encode()
	}

	// Build body
	var body io.Reader
	if req.Body != nil {
		jsonBody, err := json.Marshal(req.Body)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal request body: %w", err)
		}
		body = strings.NewReader(string(jsonBody))
	}

	// Create request
	httpReq, err := http.NewRequestWithContext(ctx, req.Method, reqURL, body)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers
	if req.Body != nil {
		httpReq.Header.Set("Content-Type", "application/json")
	}
	httpReq.Header.Set("Accept", "application/json")

	// Add custom headers
	for key, value := range req.Headers {
		httpReq.Header.Set(key, value)
	}

	// Add authentication
	token, err := c.authenticator.GetToken()
	if err == nil && token != "" {
		httpReq.Header.Set("Authorization", "APIToken "+token)
	}

	// Debug output
	if c.debug {
		fmt.Printf("--> %s %s\n", req.Method, reqURL)
		for key, values := range httpReq.Header {
			if key == "Authorization" {
				fmt.Printf("    %s: [REDACTED]\n", key)
			} else {
				fmt.Printf("    %s: %s\n", key, strings.Join(values, ", "))
			}
		}
	}

	// Execute request
	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	// Read response body
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	// Debug output
	if c.debug {
		fmt.Printf("<-- %d %s\n", resp.StatusCode, http.StatusText(resp.StatusCode))
		if len(respBody) > 0 && len(respBody) < 1000 {
			fmt.Printf("    %s\n", string(respBody))
		} else if len(respBody) >= 1000 {
			fmt.Printf("    [%d bytes]\n", len(respBody))
		}
	}

	return &Response{
		StatusCode: resp.StatusCode,
		Headers:    resp.Header,
		Body:       respBody,
	}, nil
}

// Get performs a GET request.
func (c *Client) Get(ctx context.Context, path string, query url.Values) (*Response, error) {
	return c.Do(ctx, &Request{
		Method: http.MethodGet,
		Path:   path,
		Query:  query,
	})
}

// Post performs a POST request.
func (c *Client) Post(ctx context.Context, path string, body interface{}) (*Response, error) {
	return c.Do(ctx, &Request{
		Method: http.MethodPost,
		Path:   path,
		Body:   body,
	})
}

// Put performs a PUT request.
func (c *Client) Put(ctx context.Context, path string, body interface{}) (*Response, error) {
	return c.Do(ctx, &Request{
		Method: http.MethodPut,
		Path:   path,
		Body:   body,
	})
}

// Delete performs a DELETE request.
func (c *Client) Delete(ctx context.Context, path string) (*Response, error) {
	return c.Do(ctx, &Request{
		Method: http.MethodDelete,
		Path:   path,
	})
}

// Patch performs a PATCH request.
func (c *Client) Patch(ctx context.Context, path string, body interface{}) (*Response, error) {
	return c.Do(ctx, &Request{
		Method: http.MethodPatch,
		Path:   path,
		Body:   body,
	})
}

// IsSuccess returns true if the response indicates success.
func (r *Response) IsSuccess() bool {
	return r.StatusCode >= 200 && r.StatusCode < 300
}

// DecodeJSON decodes the response body as JSON into the target.
func (r *Response) DecodeJSON(target interface{}) error {
	return json.Unmarshal(r.Body, target)
}

// Error returns an error if the response indicates failure.
func (r *Response) Error() error {
	if r.IsSuccess() {
		return nil
	}

	// Try to parse error response
	var errResp struct {
		Code    string `json:"code"`
		Message string `json:"message"`
		Details string `json:"details"`
	}

	if err := json.Unmarshal(r.Body, &errResp); err == nil && errResp.Message != "" {
		return &APIError{
			StatusCode: r.StatusCode,
			Code:       errResp.Code,
			Message:    errResp.Message,
			Details:    errResp.Details,
		}
	}

	// Return generic error
	return &APIError{
		StatusCode: r.StatusCode,
		Message:    http.StatusText(r.StatusCode),
	}
}

// APIError represents an API error response.
type APIError struct {
	StatusCode int
	Code       string
	Message    string
	Details    string
}

func (e *APIError) Error() string {
	msg := fmt.Sprintf("%d %s", e.StatusCode, e.Message)
	if e.Details != "" {
		msg += ": " + e.Details
	}
	return msg
}

// IsNotFound returns true if the error is a 404.
func (e *APIError) IsNotFound() bool {
	return e.StatusCode == http.StatusNotFound
}

// IsUnauthorized returns true if the error is a 401.
func (e *APIError) IsUnauthorized() bool {
	return e.StatusCode == http.StatusUnauthorized
}

// IsForbidden returns true if the error is a 403.
func (e *APIError) IsForbidden() bool {
	return e.StatusCode == http.StatusForbidden
}

// IsConflict returns true if the error is a 409.
func (e *APIError) IsConflict() bool {
	return e.StatusCode == http.StatusConflict
}
