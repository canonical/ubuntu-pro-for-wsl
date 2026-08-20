package certs

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"errors"
	"io"
	"testing"

	"github.com/stretchr/testify/require"
)

// newTestRootCA returns a self-signed root certificate and its key. When isCA is
// false the certificate is not a CA: it can still sign (so certificate creation
// succeeds) but anything it signs must fail verification against it.
func newTestRootCA(t *testing.T, isCA bool) (cert *x509.Certificate, key *ecdsa.PrivateKey) {
	t.Helper()

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err, "Setup: could not generate the root key")

	tmpl := template("test-root")
	tmpl.IsCA = isCA
	tmpl.KeyUsage = x509.KeyUsageCertSign
	cert, _, err = createCert(tmpl, tmpl, &key.PublicKey, key)
	require.NoError(t, err, "Setup: could not create the root certificate")

	return cert, key
}

// stubGenerateKey replaces the generateKey seam with a generator that fails on the
// n-th invocation, returning a function that restores the original behaviour.
// Tests using it must not be marked parallel: the seam is package-global.
func stubGenerateKey(failOnCall int) (restore func()) {
	original := generateKey
	calls := 0
	generateKey = func(c elliptic.Curve, r io.Reader) (*ecdsa.PrivateKey, error) {
		calls++
		if calls == failOnCall {
			return nil, errors.New("injected key generation failure")
		}
		return original(c, r)
	}
	return func() { generateKey = original }
}

func TestGenerateEphemeralPKIErrors(t *testing.T) {
	// No t.Parallel: this test stubs the package-global generateKey seam.
	testcases := map[string]struct {
		failOnCall int
	}{
		"Error when the root CA key cannot be generated":         {failOnCall: 1},
		"Error when the agent identity key cannot be generated":  {failOnCall: 2},
		"Error when the client identity key cannot be generated": {failOnCall: 3},
	}

	for name, tc := range testcases {
		t.Run(name, func(t *testing.T) {
			defer stubGenerateKey(tc.failOnCall)()

			_, err := GenerateEphemeralPKI()
			require.Error(t, err, "GenerateEphemeralPKI should have failed")
		})
	}
}

func TestIdentityCreationErrors(t *testing.T) {
	t.Parallel()

	rootCert, _ := newTestRootCA(t, true)
	mismatchingKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err, "Setup: could not generate the mismatching key")
	notCA, notCAKey := newTestRootCA(t, false)

	testcases := map[string]struct {
		cert *x509.Certificate
		key  *ecdsa.PrivateKey
	}{
		"Error when the signing key does not match the root certificate": {cert: rootCert, key: mismatchingKey},
		"Error when the root is not a CA":                                {cert: notCA, key: notCAKey},
	}

	for name, tc := range testcases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			_, err := createIdentity("agent", "agent-cn", tc.cert, tc.key)
			require.Error(t, err, "createIdentity should have failed")

			_, _, err = createClientIdentity("client-cn", tc.cert, tc.key)
			require.Error(t, err, "createClientIdentity should have failed")
		})
	}
}
