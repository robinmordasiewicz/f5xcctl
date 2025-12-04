// Package auth provides authentication mechanisms for the f5xcctl CLI.
package auth

import (
	"context"
	"crypto/rand"
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os/exec"
	"runtime"
	"strings"
	"time"
)

// TokenResponse represents an OAuth token response.
type TokenResponse struct {
	AccessToken  string    `json:"access_token"`
	TokenType    string    `json:"token_type"`
	ExpiresIn    int       `json:"expires_in"`
	RefreshToken string    `json:"refresh_token,omitempty"`
	ExpiresAt    time.Time `json:"-"`
}

// Authenticator interface for different auth methods.
type Authenticator interface {
	// GetToken returns a valid access token
	GetToken() (string, error)
	// GetHTTPClient returns an HTTP client configured for authentication
	GetHTTPClient() (*http.Client, error)
}

// TokenAuth implements authentication using an API token.
type TokenAuth struct {
	Token string
}

// NewTokenAuth creates a new token authenticator.
func NewTokenAuth(token string) *TokenAuth {
	return &TokenAuth{Token: token}
}

// GetToken returns the API token.
func (a *TokenAuth) GetToken() (string, error) {
	if a.Token == "" {
		return "", fmt.Errorf("no API token configured")
	}
	return a.Token, nil
}

// GetHTTPClient returns an HTTP client with the token header.
func (a *TokenAuth) GetHTTPClient() (*http.Client, error) {
	return &http.Client{
		Timeout: 30 * time.Second,
	}, nil
}

// CertAuth implements authentication using client certificates.
type CertAuth struct {
	CertFile string
	KeyFile  string
}

// NewCertAuth creates a new certificate authenticator.
func NewCertAuth(certFile, keyFile string) *CertAuth {
	return &CertAuth{
		CertFile: certFile,
		KeyFile:  keyFile,
	}
}

// GetToken returns an empty string for cert auth (not token-based).
func (a *CertAuth) GetToken() (string, error) {
	return "", nil
}

// GetHTTPClient returns an HTTP client configured with the client certificate.
func (a *CertAuth) GetHTTPClient() (*http.Client, error) {
	cert, err := tls.LoadX509KeyPair(a.CertFile, a.KeyFile)
	if err != nil {
		return nil, fmt.Errorf("failed to load certificate: %w", err)
	}

	tlsConfig := &tls.Config{
		Certificates: []tls.Certificate{cert},
		MinVersion:   tls.VersionTLS12,
	}

	transport := &http.Transport{
		TLSClientConfig: tlsConfig,
	}

	return &http.Client{
		Transport: transport,
		Timeout:   30 * time.Second,
	}, nil
}

// P12Auth implements authentication using PKCS#12 certificate bundles.
type P12Auth struct {
	P12File  string
	Password string
	cert     *tls.Certificate // cached parsed certificate
}

// NewP12Auth creates a new P12 authenticator.
func NewP12Auth(p12File, password string) *P12Auth {
	return &P12Auth{
		P12File:  p12File,
		Password: password,
	}
}

// GetToken returns an empty string for P12 auth (not token-based).
func (a *P12Auth) GetToken() (string, error) {
	return "", nil
}

// GetHTTPClient returns an HTTP client configured with the P12 certificate.
func (a *P12Auth) GetHTTPClient() (*http.Client, error) {
	cert, err := a.loadCertificate()
	if err != nil {
		return nil, err
	}

	tlsConfig := &tls.Config{
		Certificates: []tls.Certificate{*cert},
		MinVersion:   tls.VersionTLS12,
	}

	transport := &http.Transport{
		TLSClientConfig: tlsConfig,
	}

	return &http.Client{
		Transport: transport,
		Timeout:   30 * time.Second,
	}, nil
}

// loadCertificate loads and parses the P12 file.
func (a *P12Auth) loadCertificate() (*tls.Certificate, error) {
	if a.cert != nil {
		return a.cert, nil
	}

	cert, err := LoadP12Certificate(a.P12File, a.Password)
	if err != nil {
		return nil, err
	}

	a.cert = cert
	return cert, nil
}

// BrowserAuth implements browser-based SSO authentication.
type BrowserAuth struct {
	Tenant string
	APIURL string
}

// NewBrowserAuth creates a new browser authenticator.
func NewBrowserAuth(tenant, apiURL string) *BrowserAuth {
	return &BrowserAuth{
		Tenant: tenant,
		APIURL: apiURL,
	}
}

// Login performs browser-based authentication.
func (a *BrowserAuth) Login() (*TokenResponse, error) {
	// Generate state for CSRF protection
	state, err := generateState()
	if err != nil {
		return nil, fmt.Errorf("failed to generate state: %w", err)
	}

	// Create callback server
	listener, err := net.Listen("tcp", "127.0.0.1:0") //nolint:noctx // Simple local callback server
	if err != nil {
		return nil, fmt.Errorf("failed to create callback server: %w", err)
	}
	defer listener.Close()

	callbackURL := fmt.Sprintf("http://localhost:%d/callback", listener.Addr().(*net.TCPAddr).Port)

	// Build authorization URL
	// Note: This is a placeholder - actual F5XC OAuth endpoints may differ
	authURL := fmt.Sprintf("%s/oauth/authorize?response_type=code&client_id=f5xc-cli&redirect_uri=%s&state=%s",
		a.APIURL, url.QueryEscape(callbackURL), state)

	// Channel to receive the token
	tokenChan := make(chan *TokenResponse)
	errChan := make(chan error)

	// Start callback server
	server := &http.Server{
		ReadHeaderTimeout: 10 * time.Second,
	}
	server.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/callback" {
			http.NotFound(w, r)
			return
		}

		// Verify state
		if r.URL.Query().Get("state") != state {
			errChan <- fmt.Errorf("state mismatch - possible CSRF attack")
			http.Error(w, "State mismatch", http.StatusBadRequest)
			return
		}

		// Check for error
		if errMsg := r.URL.Query().Get("error"); errMsg != "" {
			errChan <- fmt.Errorf("authentication error: %s", errMsg)
			http.Error(w, errMsg, http.StatusBadRequest)
			return
		}

		// Get authorization code
		code := r.URL.Query().Get("code")
		if code == "" {
			errChan <- fmt.Errorf("no authorization code received")
			http.Error(w, "No code received", http.StatusBadRequest)
			return
		}

		// Exchange code for token
		token, err := a.exchangeCode(code, callbackURL)
		if err != nil {
			errChan <- err
			http.Error(w, "Token exchange failed", http.StatusInternalServerError)
			return
		}

		// Success response
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprintf(w, `<!DOCTYPE html>
<html>
<head><title>Authentication Successful</title></head>
<body>
<h1>Authentication Successful</h1>
<p>You can close this window and return to the terminal.</p>
<script>window.close();</script>
</body>
</html>`)

		tokenChan <- token
	})

	go func() {
		if err := server.Serve(listener); err != http.ErrServerClosed {
			errChan <- err
		}
	}()

	// Open browser
	if err := openBrowser(authURL); err != nil {
		return nil, fmt.Errorf("failed to open browser: %w", err)
	}

	// Wait for callback or timeout
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	select {
	case token := <-tokenChan:
		_ = server.Shutdown(context.Background())
		return token, nil
	case err := <-errChan:
		_ = server.Shutdown(context.Background())
		return nil, err
	case <-ctx.Done():
		_ = server.Shutdown(context.Background())
		return nil, fmt.Errorf("authentication timed out")
	}
}

// exchangeCode exchanges an authorization code for tokens.
func (a *BrowserAuth) exchangeCode(code, callbackURL string) (*TokenResponse, error) {
	// Note: This is a placeholder - actual token exchange may differ
	tokenURL := fmt.Sprintf("%s/oauth/token", a.APIURL)

	data := url.Values{}
	data.Set("grant_type", "authorization_code")
	data.Set("code", code)
	data.Set("redirect_uri", callbackURL)
	data.Set("client_id", "f5xc-cli")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, tokenURL, strings.NewReader(data.Encode()))
	if err != nil {
		return nil, fmt.Errorf("failed to create token request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := http.DefaultClient.Do(req) //nolint:gosec // G107: URL is constructed from known config
	if err != nil {
		return nil, fmt.Errorf("token request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("token request failed with status %d", resp.StatusCode)
	}

	var token TokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&token); err != nil {
		return nil, fmt.Errorf("failed to decode token response: %w", err)
	}

	// Set expiration time
	if token.ExpiresIn > 0 {
		token.ExpiresAt = time.Now().Add(time.Duration(token.ExpiresIn) * time.Second)
	}

	return &token, nil
}

// generateState generates a random state string for CSRF protection.
func generateState() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(b), nil
}

// openBrowser opens the URL in the default browser.
func openBrowser(targetURL string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	var cmd *exec.Cmd

	switch runtime.GOOS {
	case "darwin":
		cmd = exec.CommandContext(ctx, "open", targetURL)
	case "linux":
		cmd = exec.CommandContext(ctx, "xdg-open", targetURL)
	case "windows":
		cmd = exec.CommandContext(ctx, "rundll32", "url.dll,FileProtocolHandler", targetURL)
	default:
		return fmt.Errorf("unsupported platform: %s", runtime.GOOS)
	}

	return cmd.Start()
}
