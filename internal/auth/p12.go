package auth

import (
	"crypto/tls"
	"fmt"
	"os"

	"software.sslmate.com/src/go-pkcs12"
)

// LoadP12Certificate loads a PKCS#12 (.p12/.pfx) certificate file.
func LoadP12Certificate(p12File, password string) (*tls.Certificate, error) {
	// Read the P12 file
	p12Data, err := os.ReadFile(p12File)
	if err != nil {
		return nil, fmt.Errorf("failed to read P12 file: %w", err)
	}

	// Decode the P12 data
	privateKey, certificate, caCerts, err := pkcs12.DecodeChain(p12Data, password)
	if err != nil {
		return nil, fmt.Errorf("failed to decode P12 file: %w", err)
	}

	// Build the certificate chain
	cert := tls.Certificate{
		Certificate: [][]byte{certificate.Raw},
		PrivateKey:  privateKey,
		Leaf:        certificate,
	}

	// Add CA certificates to the chain
	for _, ca := range caCerts {
		cert.Certificate = append(cert.Certificate, ca.Raw)
	}

	return &cert, nil
}
