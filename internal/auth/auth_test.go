package auth

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewTokenAuth(t *testing.T) {
	auth := NewTokenAuth("test-token")
	assert.NotNil(t, auth)
	assert.Equal(t, "test-token", auth.Token)
}

func TestTokenAuth_GetToken(t *testing.T) {
	tests := []struct {
		name      string
		token     string
		wantToken string
		wantErr   bool
	}{
		{
			name:      "valid token",
			token:     "my-api-token",
			wantToken: "my-api-token",
			wantErr:   false,
		},
		{
			name:      "empty token",
			token:     "",
			wantToken: "",
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			auth := NewTokenAuth(tt.token)
			token, err := auth.GetToken()

			if tt.wantErr {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), "no API token configured")
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.wantToken, token)
			}
		})
	}
}

func TestTokenAuth_GetHTTPClient(t *testing.T) {
	auth := NewTokenAuth("test-token")
	client, err := auth.GetHTTPClient()

	assert.NoError(t, err)
	assert.NotNil(t, client)
	assert.Equal(t, 30*time.Second, client.Timeout)
}

func TestNewCertAuth(t *testing.T) {
	auth := NewCertAuth("/path/to/cert.pem", "/path/to/key.pem")
	assert.NotNil(t, auth)
	assert.Equal(t, "/path/to/cert.pem", auth.CertFile)
	assert.Equal(t, "/path/to/key.pem", auth.KeyFile)
}

func TestCertAuth_GetToken(t *testing.T) {
	auth := NewCertAuth("cert.pem", "key.pem")
	token, err := auth.GetToken()

	assert.NoError(t, err)
	assert.Equal(t, "", token) // Cert auth doesn't use tokens
}

func TestCertAuth_GetHTTPClient_Success(t *testing.T) {
	// Create temporary test certificate and key
	certFile, keyFile, cleanup := createTestCertificate(t)
	defer cleanup()

	auth := NewCertAuth(certFile, keyFile)
	client, err := auth.GetHTTPClient()

	assert.NoError(t, err)
	assert.NotNil(t, client)
	assert.Equal(t, 30*time.Second, client.Timeout)
	assert.NotNil(t, client.Transport)
}

func TestCertAuth_GetHTTPClient_InvalidCert(t *testing.T) {
	auth := NewCertAuth("/nonexistent/cert.pem", "/nonexistent/key.pem")
	client, err := auth.GetHTTPClient()

	assert.Error(t, err)
	assert.Nil(t, client)
	assert.Contains(t, err.Error(), "failed to load certificate")
}

func TestNewBrowserAuth(t *testing.T) {
	auth := NewBrowserAuth("my-tenant", "https://api.example.com")
	assert.NotNil(t, auth)
	assert.Equal(t, "my-tenant", auth.Tenant)
	assert.Equal(t, "https://api.example.com", auth.APIURL)
}

func TestGenerateState(t *testing.T) {
	// Test that generateState produces unique, non-empty values
	states := make(map[string]bool)

	for i := 0; i < 100; i++ {
		state, err := generateState()
		assert.NoError(t, err)
		assert.NotEmpty(t, state)
		assert.Len(t, state, 44) // Base64 encoded 32 bytes = 44 chars

		// Ensure uniqueness
		assert.False(t, states[state], "duplicate state generated")
		states[state] = true
	}
}

func TestTokenResponse_Struct(t *testing.T) {
	token := TokenResponse{
		AccessToken:  "access-123",
		TokenType:    "Bearer",
		ExpiresIn:    3600,
		RefreshToken: "refresh-456",
		ExpiresAt:    time.Now().Add(time.Hour),
	}

	assert.Equal(t, "access-123", token.AccessToken)
	assert.Equal(t, "Bearer", token.TokenType)
	assert.Equal(t, 3600, token.ExpiresIn)
	assert.Equal(t, "refresh-456", token.RefreshToken)
	assert.False(t, token.ExpiresAt.IsZero())
}

func TestAuthenticatorInterface(t *testing.T) {
	// Verify both auth types implement Authenticator interface
	var _ Authenticator = (*TokenAuth)(nil)
	var _ Authenticator = (*CertAuth)(nil)
}

// Helper function to create a test certificate and key.
func createTestCertificate(t *testing.T) (certFile, keyFile string, cleanup func()) {
	t.Helper()

	// Generate a private key
	privateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)

	// Create certificate template
	template := x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			Organization: []string{"Test Org"},
			CommonName:   "test.example.com",
		},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().Add(time.Hour),
		KeyUsage:              x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
		BasicConstraintsValid: true,
	}

	// Create self-signed certificate
	certDER, err := x509.CreateCertificate(rand.Reader, &template, &template, &privateKey.PublicKey, privateKey)
	require.NoError(t, err)

	// Create temp files
	certF, err := os.CreateTemp("", "test-cert-*.pem")
	require.NoError(t, err)

	keyF, err := os.CreateTemp("", "test-key-*.pem")
	require.NoError(t, err)

	// Write certificate
	err = pem.Encode(certF, &pem.Block{Type: "CERTIFICATE", Bytes: certDER})
	require.NoError(t, err)
	certF.Close()

	// Write private key
	keyDER, err := x509.MarshalECPrivateKey(privateKey)
	require.NoError(t, err)
	err = pem.Encode(keyF, &pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER})
	require.NoError(t, err)
	keyF.Close()

	cleanup = func() {
		os.Remove(certF.Name())
		os.Remove(keyF.Name())
	}

	return certF.Name(), keyF.Name(), cleanup
}
