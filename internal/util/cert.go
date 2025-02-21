package util

import (
	"context"
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

	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// GenerateSelfSignedCert generates a new self-signed certificate and writes the files to certDir.
func GenerateSelfSignedCert(certDir, commonName string) error {
	keyPath := filepath.Join(certDir, "tls.key")
	certPath := filepath.Join(certDir, "tls.crt")

	// Generate a new RSA private key.
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return fmt.Errorf("failed to generate private key: %w", err)
	}

	// Create a certificate template with SANs.
	template := x509.Certificate{
		SerialNumber: big.NewInt(time.Now().UnixNano()),
		Subject: pkix.Name{
			CommonName: commonName,
		},
		DNSNames:              []string{commonName},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().Add(365 * 24 * time.Hour), // 1 year validity
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
	}

	// Self-sign the certificate.
	derBytes, err := x509.CreateCertificate(rand.Reader, &template, &template, &priv.PublicKey, priv)
	if err != nil {
		return fmt.Errorf("failed to create certificate: %w", err)
	}

	// Write certificate to file.
	certOut, err := os.Create(certPath)
	if err != nil {
		return fmt.Errorf("failed to open %s for writing: %w", certPath, err)
	}
	defer certOut.Close()
	if err := pem.Encode(certOut, &pem.Block{Type: "CERTIFICATE", Bytes: derBytes}); err != nil {
		return fmt.Errorf("failed to write certificate to %s: %w", certPath, err)
	}

	// Write private key to file.
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

// writeCertFiles writes certificate and key data into files in certDir.
func writeCertFiles(certDir string, certData, keyData []byte) error {
	certPath := filepath.Join(certDir, "tls.crt")
	keyPath := filepath.Join(certDir, "tls.key")

	if err := os.WriteFile(certPath, certData, 0644); err != nil {
		return fmt.Errorf("failed to write certificate to %s: %w", certPath, err)
	}
	if err := os.WriteFile(keyPath, keyData, 0600); err != nil {
		return fmt.Errorf("failed to write key to %s: %w", keyPath, err)
	}
	return nil
}

// LoadOrGenerateCert checks for an existing cert secret and writes its contents to certDir.
// If the secret doesn't exist, it generates a new certificate and creates the secret.
func LoadOrGenerateCert(ctx context.Context, c client.Client, namespace, secretName, certDir, commonName string) error {
	// Ensure the cert directory exists.
	if err := os.MkdirAll(certDir, 0755); err != nil {
		return fmt.Errorf("failed to create cert directory %s: %w", certDir, err)
	}

	secret := &corev1.Secret{}
	err := c.Get(ctx, client.ObjectKey{Namespace: namespace, Name: secretName}, secret)
	if err == nil {
		// Secret exists; extract certificate and key.
		certData, certOk := secret.Data["tls.crt"]
		keyData, keyOk := secret.Data["tls.key"]
		if !certOk || !keyOk {
			return fmt.Errorf("secret %s exists but does not contain tls.crt and tls.key", secretName)
		}
		// Write certificate and key files to certDir.
		return writeCertFiles(certDir, certData, keyData)
	} else if apierrors.IsNotFound(err) {
		// Secret does not exist; generate a new certificate.
		if err := GenerateSelfSignedCert(certDir, commonName); err != nil {
			return fmt.Errorf("failed to generate self-signed certificate: %w", err)
		}
		// Read the generated certificate and key.
		certData, err := os.ReadFile(filepath.Join(certDir, "tls.crt"))
		if err != nil {
			return fmt.Errorf("failed to read generated certificate: %w", err)
		}
		keyData, err := os.ReadFile(filepath.Join(certDir, "tls.key"))
		if err != nil {
			return fmt.Errorf("failed to read generated key: %w", err)
		}
		// Create the certificate secret.
		newSecret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      secretName,
				Namespace: namespace,
			},
			Data: map[string][]byte{
				"tls.crt": certData,
				"tls.key": keyData,
			},
			Type: corev1.SecretTypeTLS,
		}
		if err := c.Create(ctx, newSecret); err != nil {
			return fmt.Errorf("failed to create certificate secret: %w", err)
		}
		return nil
	} else {
		return fmt.Errorf("failed to get certificate secret: %w", err)
	}
}

func UpdateWebhookCABundle(ctx context.Context, c client.Client, webhookName string, vwcName string, secretNamespace, secretName string) error {
	// Retrieve the TLS secret.
	secret := &corev1.Secret{}
	if err := c.Get(ctx, client.ObjectKey{Namespace: secretNamespace, Name: secretName}, secret); err != nil {
		return fmt.Errorf("failed to get secret %s/%s: %w", secretNamespace, secretName, err)
	}
	caBundle, ok := secret.Data["tls.crt"]
	if !ok || len(caBundle) == 0 {
		return fmt.Errorf("secret %s/%s does not contain a valid tls.crt", secretNamespace, secretName)
	}

	// Retrieve the ValidatingWebhookConfiguration.
	vwc := &admissionregistrationv1.ValidatingWebhookConfiguration{}
	if err := c.Get(ctx, client.ObjectKey{Name: vwcName}, vwc); err != nil {
		return fmt.Errorf("failed to get ValidatingWebhookConfiguration %s: %w", vwcName, err)
	}

	// Update the CA bundle for the webhook matching webhookName.
	updated := false
	for i, wh := range vwc.Webhooks {
		if wh.Name == webhookName {
			vwc.Webhooks[i].ClientConfig.CABundle = caBundle
			updated = true
		}
	}
	if !updated {
		return fmt.Errorf("webhook with name %q not found in ValidatingWebhookConfiguration %s", webhookName, vwcName)
	}

	// Apply the update.
	if err := c.Update(ctx, vwc); err != nil {
		return fmt.Errorf("failed to update ValidatingWebhookConfiguration %s: %w", vwcName, err)
	}
	return nil
}
