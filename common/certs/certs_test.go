package certs_test

import (
	"math/big"
	"os"
	"path/filepath"
	"testing"

	"github.com/canonical/ubuntu-pro-for-wsl/common/certs"
	"github.com/stretchr/testify/require"
)

func TestCreateTLSCertificateSignedBy(t *testing.T) {
	t.Parallel()

	testcases := map[string]struct {
		missingSerialNumber bool
		rootIsNotCA         bool
		breakCertPem        bool
		breakKeyPem         bool

		wantErr bool
	}{
		"Success": {},

		"Error when serial number is missing":                    {missingSerialNumber: true, wantErr: true},
		"Error when the signing certificate is not an authority": {rootIsNotCA: true, wantErr: true},
		"Error when the cert.pem file cannot be written":         {breakCertPem: true, wantErr: true},
		"Error when the key.pem file cannot be written":          {breakKeyPem: true, wantErr: true},
	}

	for name, tc := range testcases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			rootCert, rootKey, err := certs.CreateRootCA("test-root-ca", new(big.Int).SetInt64(1), t.TempDir())
			require.NoError(t, err, "Setup: failed to generate root CA cert")
			if tc.rootIsNotCA {
				rootCert.IsCA = false
				rootCert.AuthorityKeyId = nil
			}

			var testSerial *big.Int
			if !tc.missingSerialNumber {
				testSerial = new(big.Int).SetInt64(1)
			}

			dir := t.TempDir()

			if tc.breakCertPem {
				require.NoError(t, os.MkdirAll(filepath.Join(dir, "server_cert.pem"), 0700), "Setup: failed to create a directory that should break cert.pem")
			}

			if tc.breakKeyPem {
				require.NoError(t, os.MkdirAll(filepath.Join(dir, "server_key.pem"), 0700), "Setup: failed to create a directory that should break key.pem")
			}

			tlsCert, err := certs.CreateTLSCertificateSignedBy("server", "test-server-cn", testSerial, rootCert, rootKey, dir)

			if tc.wantErr {
				require.Error(t, err, "CreateTLSCertificateSignedBy should have failed")
				return
			}
			require.NoError(t, err, "CreateTLSCertificateSignedBy failed")
			require.NotNil(t, tlsCert, "CreateTLSCertificateSignedBy returned a nil certificate")
			require.FileExists(t, filepath.Join(dir, "server_cert.pem"), "CreateTLSCertificateSignedBy failed to write the certificate")
			require.FileExists(t, filepath.Join(dir, "server_key.pem"), "CreateTLSCertificateSignedBy failed to write the certificate")
		})
	}
}

func TestCreateRooCA(t *testing.T) {
	t.Parallel()

	testcases := map[string]struct {
		missingSerialNumber bool
		breakCertPem        bool

		wantErr bool
	}{
		"Success": {},

		"Error when serial number is missing":               {missingSerialNumber: true, wantErr: true},
		"Error when the ca_cert.pem file cannot be written": {breakCertPem: true, wantErr: true},
	}

	for name, tc := range testcases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			var testSerial *big.Int
			if !tc.missingSerialNumber {
				testSerial = new(big.Int).SetInt64(1)
			}

			dir := t.TempDir()

			if tc.breakCertPem {
				require.NoError(t, os.MkdirAll(filepath.Join(dir, "ca_cert.pem"), 0700), "Setup: failed to create a directory that should break cert.pem")
			}

			rootCert, rootKey, err := certs.CreateRootCA("test-root-ca", testSerial, dir)

			if tc.wantErr {
				require.Error(t, err, "CreateRootCA should have failed")
				return
			}
			require.NoError(t, err, "CreateRootCA failed")
			require.NotNil(t, rootCert, "CreateRootCA didn't return a certificate")
			require.NotNil(t, rootKey, "CreateRootCA didn't return a private key")
			require.FileExists(t, filepath.Join(dir, "ca_cert.pem"), "CreateRootCA failed to write the certificate to disk")
		})
	}
}
