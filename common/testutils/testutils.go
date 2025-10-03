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
func GenerateTempCertificate(t *testing.T, path string) {
	t.Helper()

	const errPrefix = "Setup: could not generate temporary certificate: "

	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err, errPrefix+"could not generate keys")

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
	require.NoError(t, err, errPrefix+"could not create certificate")

	// Marshal and write certificate
	out := &bytes.Buffer{}
	err = pem.Encode(out, &pem.Block{Type: "CERTIFICATE", Bytes: cert})
	require.NoError(t, err, errPrefix+"could not encode certificate")

	err = os.WriteFile(filepath.Join(path, "cert.pem"), out.Bytes(), 0600)
	require.NoError(t, err, errPrefix+"could not write certificate to file")

	// Marshal and write private key
	key, err := x509.MarshalECPrivateKey(priv)
	require.NoError(t, err, errPrefix+"could not marshal private key")

	out = &bytes.Buffer{}
	err = pem.Encode(out, &pem.Block{Type: "EC PRIVATE KEY", Bytes: key})
	require.NoError(t, err, errPrefix+"could not encode private key")

	err = os.WriteFile(filepath.Join(path, "key.pem"), out.Bytes(), 0600)
	require.NoError(t, err, errPrefix+"could not write private key to file")
}

// WriteOsRelease is a test helper that writes a sample os-release file for the given distro inside
// the uncRoot directory, creating any needed directories.
func WriteOsRelease(t *testing.T, uncRoot, distroName, template string) {
	t.Helper()

	testdata, err := filepath.Abs(filepath.Join(TestFamilyPath(t), template))
	require.NoError(t, err, "Setup: Couldn't compute absolute path of sample os-release file")
	data, err := os.ReadFile(testdata)
	require.NoError(t, err, "Setup: couldn't read sample os-release file")

	dir := filepath.Join(uncRoot, distroName, "usr", "lib")
	err = os.MkdirAll(dir, 0750)
	require.NoError(t, err, "Setup: Failed to create directories to contain the distros os-release file")

	//nolint:gosec // This file is meant to be read by anyone.
	err = os.WriteFile(filepath.Join(dir, "os-release"), data, 0644)
	require.NoError(t, err, "Setup: Failed to write sample os-release file")
}
