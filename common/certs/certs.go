// Package certs provides functions to create certificates suitable for mTLS communication.
// In production only the agent should create those certificates, but placing this in the common module facilities other components's tests.
package certs

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"time"

	"github.com/canonical/ubuntu-pro-for-wsl/common"
	"github.com/ubuntu/decorate"
)

// MinTLSVersion is the minimum TLS version used for the PKI-generated mTLS connections.
//
// We pin TLS 1.2 rather than 1.3 because the mTLS channel must work on Windows 10 hosts. We keep the
// same minimum on both sides to avoid handshake surprises.
const MinTLSVersion = tls.VersionTLS12

// PKI represents an ephemeral Public Key Infrastructure instance. Consume the bits you need at
// startup and drop it.
type PKI struct {
	AgentTLSConfig *tls.Config
	PEMFiles       map[string][]byte
}

// generateKey is assigned to a package-level variable so tests can simulate key generation failures.
var generateKey = ecdsa.GenerateKey

// GenerateEphemeralPKI creates a self-signed root CA authority, agent identity, and shared client identity.
// It returns a PKI holding the agent's TLS config (in memory) and the publishable byte streams for clients.
//
// We use ECDSA P-256 rather than Ed25519 because Flutter's BoringSSL-based TLS stack does not support
// Ed25519 certificates when the connection is constrained to TLS 1.2.
func GenerateEphemeralPKI() (pki PKI, err error) {
	defer decorate.OnError(&err, "could not generate ephemeral PKI:")

	rootKey, err := generateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return PKI{}, fmt.Errorf("failed to generate CA key: %v", err)
	}

	rootCertTmpl := template(common.GRPCServerNameOverride)
	rootCertTmpl.IsCA = true
	rootCertTmpl.Subject.CommonName = common.GRPCServerNameOverride + " CA"
	rootCertTmpl.KeyUsage = x509.KeyUsageCertSign

	// We pass the template as the parent as well so that the certificate is self-signed.
	rootCert, rootDER, err := createCert(rootCertTmpl, rootCertTmpl, &rootKey.PublicKey, rootKey)
	if err != nil {
		return PKI{}, fmt.Errorf("failed to create root CA cert: %v", err)
	}

	agentTLS, err := createIdentity(common.AgentCertFilePrefix, common.GRPCServerNameOverride, rootCert, rootKey)
	if err != nil {
		return PKI{}, fmt.Errorf("failed to create agent certificate: %v", err)
	}

	clientCertDER, clientKeyPEM, err := createClientIdentity(common.GRPCServerNameOverride, rootCert, rootKey)
	if err != nil {
		return PKI{}, fmt.Errorf("failed to create client certificate: %v", err)
	}

	caPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: rootDER})
	clientCertPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: clientCertDER})

	caCertPool := x509.NewCertPool()
	caCertPool.AddCert(rootCert)

	agentTLSConfig := &tls.Config{
		Certificates: []tls.Certificate{*agentTLS},
		ClientCAs:    caCertPool,
		ClientAuth:   tls.RequireAndVerifyClientCert,
		MinVersion:   MinTLSVersion,
	}

	return PKI{
		AgentTLSConfig: agentTLSConfig,
		PEMFiles: map[string][]byte{
			common.RootCACertFileName:                               caPEM,
			common.ClientsCertFilePrefix + common.CertificateSuffix: clientCertPEM,
			common.ClientsCertFilePrefix + common.KeySuffix:         clientKeyPEM,
		},
	}, nil
}

func createIdentity(name, certCN string, rootCACert *x509.Certificate, rootCAKey *ecdsa.PrivateKey) (*tls.Certificate, error) {
	key, err := generateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("failed to generate key for %s: %v", name, err)
	}

	certTmpl := template(certCN)
	// Customizing the usage for client and server applications:
	// Even though x509.CreateCertificate documentation says it will use it, if present,
	// it seems we need to set AuthorityKeyId manually to make the verification work.
	certTmpl.KeyUsage = x509.KeyUsageDigitalSignature | x509.KeyUsageKeyAgreement | x509.KeyUsageKeyEncipherment
	certTmpl.ExtKeyUsage = []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth}
	certTmpl.AuthorityKeyId = rootCACert.SubjectKeyId

	cert, der, err := createCert(certTmpl, rootCACert, &key.PublicKey, rootCAKey)
	if err != nil {
		return nil, err
	}

	// Verify the certificate against the root certificate.
	caCertPool := x509.NewCertPool()
	caCertPool.AddCert(rootCACert)
	if _, err = cert.Verify(x509.VerifyOptions{Roots: caCertPool}); err != nil {
		return nil, fmt.Errorf("certificate verification failed: %v", err)
	}

	return &tls.Certificate{
		Certificate: [][]byte{der},
		PrivateKey:  key,
		Leaf:        cert,
	}, nil
}

func createClientIdentity(certCN string, rootCACert *x509.Certificate, rootCAKey *ecdsa.PrivateKey) (der []byte, keyPEM []byte, err error) {
	key, err := generateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to generate key for client: %v", err)
	}

	certTmpl := template(certCN)
	certTmpl.KeyUsage = x509.KeyUsageDigitalSignature | x509.KeyUsageKeyAgreement | x509.KeyUsageKeyEncipherment
	certTmpl.ExtKeyUsage = []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth}
	certTmpl.AuthorityKeyId = rootCACert.SubjectKeyId

	cert, der, err := createCert(certTmpl, rootCACert, &key.PublicKey, rootCAKey)
	if err != nil {
		return nil, nil, err
	}

	caCertPool := x509.NewCertPool()
	caCertPool.AddCert(rootCACert)
	if _, err = cert.Verify(x509.VerifyOptions{Roots: caCertPool}); err != nil {
		return nil, nil, fmt.Errorf("certificate verification failed: %v", err)
	}

	pkcs8Key, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to marshal client key: %v", err)
	}
	keyPEM = pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: pkcs8Key})

	return der, keyPEM, nil
}

// createCert invokes x509.CreateCertificate and returns the certificate and it's DER as bytes for serialization.
func createCert(template, parent *x509.Certificate, pub, parentPriv any) (cert *x509.Certificate, certDER []byte, err error) {
	defer decorate.OnError(&err, "could not create certificate:")

	certDER, err = x509.CreateCertificate(rand.Reader, template, parent, pub, parentPriv)
	if err != nil {
		return nil, nil, err
	}

	// parse the resulting certificate so we can use it again
	cert, err = x509.ParseCertificate(certDER)
	return cert, certDER, err
}

// template is a helper function to create a cert template with required fields filled in for UP4W specific use case.
func template(commonName string) *x509.Certificate {
	return &x509.Certificate{
		Subject:               pkix.Name{Organization: []string{commonName}, CommonName: commonName},
		DNSNames:              []string{commonName, "localhost", "127.0.0.1"},
		SignatureAlgorithm:    x509.ECDSAWithSHA256,
		NotBefore:             time.Now(),
		NotAfter:              time.Now().Add(time.Hour * 24 * 30), // arbitrarily chosen expiration in a month
		BasicConstraintsValid: true,
	}
}
