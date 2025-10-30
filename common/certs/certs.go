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
	"os"
	"path/filepath"
	"time"

	"github.com/canonical/ubuntu-pro-for-wsl/common"
	"github.com/ubuntu/decorate"
)

// Heavily inspired in:
// - https://go.dev/src/crypto/tls/generate_cert.go
// - https://github.com/grpc/grpc-go/blob/master/examples/features/encryption/mTLS
// - and https://gist.github.com/annanay25/43e3846e21b30818d8dcd5f9987e852d.

// CreateRootCA creates a new root certificate authority (CA) certificate and private key pair with the common name provided.
// Only the cert is written into destDir in the PEM format. Being a CA, the certificate and private key returned can be used to sign other certificates.
func CreateRootCA(commonName string, destDir string) (rootCert *x509.Certificate, rootKey *ecdsa.PrivateKey, err error) {
	// generate a new key-pair for the root certificate based on the P256 elliptic curve
	rootKey, err = ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to generate random key: %v", err)
	}

	rootCertTmpl := template(commonName)
	rootCertTmpl.IsCA = true
	rootCertTmpl.Subject.CommonName = commonName + " CA"
	rootCertTmpl.KeyUsage = x509.KeyUsageCertSign

	// We pass the template as the parent as well so that the certificate is self-signed.
	rootCert, rootDER, err := createCert(rootCertTmpl, rootCertTmpl, &rootKey.PublicKey, rootKey)
	if err != nil {
		return nil, nil, err
	}

	// Write the CA certificate to disk.
	// Notice that we don't write the private key to disk. Only the caller of this function can create other certificates signed by this root CA.
	if err = writeCert(filepath.Join(destDir, common.RootCACertFileName), rootDER); err != nil {
		return nil, nil, err
	}

	return rootCert, rootKey, nil
}

// CreateTLSCertificateSignedBy creates a certificate and key pair usable for authentication signed by the root certificate authority (root CA) certificate and key provided and write them into destDir in the PEM format.
func CreateTLSCertificateSignedBy(name, certCN string, rootCACert *x509.Certificate, rootCAKey *ecdsa.PrivateKey, destDir string) (tlsCert *tls.Certificate, err error) {
	decorate.OnError(&err, "could not create root signed certificate pair for %s:", name)

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("failed to generate random key: %v", err)
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

	if err = writeCert(filepath.Join(destDir, name+common.CertificateSuffix), der); err != nil {
		return nil, err
	}
	if err = writeKey(filepath.Join(destDir, name+common.KeySuffix), key); err != nil {
		return nil, err
	}

	return &tls.Certificate{
		Certificate: [][]byte{der},
		PrivateKey:  key,
		Leaf:        cert,
	}, nil
}

// createCert invokes x509.CreateCertificate and returns the certificate and it's DER as bytes for serialization.
func createCert(template, parent *x509.Certificate, pub, parentPriv any) (cert *x509.Certificate, certDER []byte, err error) {
	decorate.OnError(&err, "could not create certificate:")

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

// writeCert writes a certificate to disk in PEM format to the given filename.
func writeCert(filename string, DER []byte) error {
	w, err := os.Create(filename)
	if err != nil {
		return fmt.Errorf("failed to open %q for writing: %v", filename, err)
	}
	defer w.Close()

	return pem.Encode(w, &pem.Block{Type: "CERTIFICATE", Bytes: DER})
}

// writeKey writes a private key to disk in PEM format to the given filename.
func writeKey(filename string, priv *ecdsa.PrivateKey) error {
	w, err := os.OpenFile(filename, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return fmt.Errorf("failed to open %q for writing: %v", filename, err)
	}
	defer w.Close()

	p, err := x509.MarshalPKCS8PrivateKey(priv)
	if err != nil {
		return fmt.Errorf("failed to marshal private key: %v", err)
	}

	return pem.Encode(w, &pem.Block{Type: "PRIVATE KEY", Bytes: p})
}
