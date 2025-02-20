package util

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"os"
	"path/filepath"
	"time"
)

// generateSelfSignedCert generates a self‑signed certificate if one doesn’t exist.
func GenerateSelfSignedCert(certDir, commonName string) error {
	keyPath := filepath.Join(certDir, "tls.key")
	certPath := filepath.Join(certDir, "tls.crt")

	// Generate RSA private key.
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return fmt.Errorf("failed to generate private key: %w", err)
	}

	// Create certificate template.
	template := x509.Certificate{
		SerialNumber: big.NewInt(time.Now().UnixNano()),
		Subject: pkix.Name{
			CommonName: commonName,
		},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().Add(365 * 24 * time.Hour), // 1 year
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
	}

	// Create the self-signed certificate.
	derBytes, err := x509.CreateCertificate(rand.Reader, &template, &template, &priv.PublicKey, priv)
	if err != nil {
		return fmt.Errorf("failed to create certificate: %w", err)
	}

	// Write certificate.
	certOut, err := os.Create(certPath)
	if err != nil {
		return fmt.Errorf("failed to open %s for writing: %w", certPath, err)
	}
	defer certOut.Close()
	if err := pem.Encode(certOut, &pem.Block{Type: "CERTIFICATE", Bytes: derBytes}); err != nil {
		return fmt.Errorf("failed to write certificate to %s: %w", certPath, err)
	}

	// Write private key.
	keyOut, err := os.Create(keyPath)
	if err != nil {
		return fmt.Errorf("failed to open %s for writing: %w", keyPath, err)
	}
	defer keyOut.Close()
	if err := pem.Encode(keyOut, &pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(priv)}); err != nil {
		return fmt.Errorf("failed to write private key to %s: %w", keyPath, err)
	}

	return nil
}
