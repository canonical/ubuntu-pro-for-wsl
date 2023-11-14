// Package testutils implements helper functions for frequently needed functionality
// in tests.
package testutils

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// ReplaceFileWithDir removes a file and creates a directory with the same path.
// Useful to break file reads and assert on the errors.
func ReplaceFileWithDir(t *testing.T, path string, msg string, args ...any) {
	t.Helper()

	if err := os.RemoveAll(path); err != nil {
		err = fmt.Errorf("could not remove file: %v", err)
		require.NoErrorf(t, err, msg, args...)
	}

	if err := os.MkdirAll(path, 0700); err != nil {
		err = fmt.Errorf("could not create folder at file's location: %v", err)
		require.NoErrorf(t, err, msg, args...)
	}
}

// GenerateTempCertificate generates a self-signed certificate valid for one hour. Both the
// certificate and the private key are stored in the specified path.
func GenerateTempCertificate(path string) error {
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return fmt.Errorf("could not generate keys: %v", err)
	}

	template := x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			CommonName:   "CanonicalGroupLimited",
			Country:      []string{"US"},
			Organization: []string{"Canonical"},
		},
		IPAddresses: []net.IP{net.ParseIP("127.0.0.1")},
		NotBefore:   time.Now(),
		NotAfter:    time.Now().Add(time.Hour),
	}

	cert, err := x509.CreateCertificate(rand.Reader, &template, &template, &priv.PublicKey, priv)
	if err != nil {
		return fmt.Errorf("could not create certificate: %s", err)
	}

	// Marshal and write certificate
	out := &bytes.Buffer{}
	if err := pem.Encode(out, &pem.Block{Type: "CERTIFICATE", Bytes: cert}); err != nil {
		return fmt.Errorf("could not encode certificate: %v", err)
	}

	if err := os.WriteFile(filepath.Join(path, "cert.pem"), out.Bytes(), 0600); err != nil {
		return fmt.Errorf("could not write certificate to file: %v", err)
	}

	// Marshal and write private key
	key, err := x509.MarshalECPrivateKey(priv)
	if err != nil {
		return fmt.Errorf("could not marshal private key: %v", err)
	}

	out = &bytes.Buffer{}
	if err := pem.Encode(out, &pem.Block{Type: "EC PRIVATE KEY", Bytes: key}); err != nil {
		return fmt.Errorf("could not encode private key: %v", err)
	}

	if err := os.WriteFile(filepath.Join(path, "key.pem"), out.Bytes(), 0600); err != nil {
		return fmt.Errorf("could not write private key to file: %v", err)
	}

	return nil
}
