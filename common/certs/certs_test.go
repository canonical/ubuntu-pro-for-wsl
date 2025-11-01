package certs_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/canonical/ubuntu-pro-for-wsl/common"
	"github.com/canonical/ubuntu-pro-for-wsl/common/certs"
	"github.com/stretchr/testify/require"
)

func TestCreateRooCA(t *testing.T) {
	t.Parallel()

	testcases := map[string]struct {
		breakCertPem bool

		wantErr bool
	}{
		"Success": {},

		"Error when the root CA certificate file cannot be written": {breakCertPem: true, wantErr: true},
	}

	for name, tc := range testcases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			dir := t.TempDir()

			if tc.breakCertPem {
				require.NoError(t, os.MkdirAll(filepath.Join(dir, common.RootCACertFileName), 0700), "Setup: failed to create a directory that should break cert.pem")
			}

			rootCert, rootKey, err := certs.CreateRootCA("test-root-ca", dir)

			if tc.wantErr {
				require.Error(t, err, "CreateRootCA should have failed")
				return
			}
			require.NoError(t, err, "CreateRootCA failed")
			require.NotNil(t, rootCert, "CreateRootCA didn't return a certificate")
			require.NotNil(t, rootKey, "CreateRootCA didn't return a private key")
			require.FileExists(t, filepath.Join(dir, common.RootCACertFileName), "CreateRootCA failed to write the certificate to disk")
		})
	}
}

func TestCreateTLSCertificateSignedBy(t *testing.T) {
	t.Parallel()

	testcases := map[string]struct {
		rootIsNotCA  bool
		breakCertPem bool
		breakKeyPem  bool

		wantErr bool
	}{
		"Success": {},

		"Error when the signing certificate is not an authority": {rootIsNotCA: true, wantErr: true},
		"Error when the cert.pem file cannot be written":         {breakCertPem: true, wantErr: true},
		"Error when the key.pem file cannot be written":          {breakKeyPem: true, wantErr: true},
	}

	for name, tc := range testcases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			rootCert, rootKey, err := certs.CreateRootCA("test-root-ca", t.TempDir())
			require.NoError(t, err, "Setup: failed to generate root CA cert")
			if tc.rootIsNotCA {
				rootCert.IsCA = false
				rootCert.AuthorityKeyId = nil
			}

			dir := t.TempDir()

			agentCertName := common.AgentCertFilePrefix + common.CertificateSuffix
			agentKeyName := common.AgentCertFilePrefix + common.KeySuffix

			if tc.breakCertPem {
				require.NoError(t, os.MkdirAll(filepath.Join(dir, agentCertName), 0700), "Setup: failed to create a directory that should break cert.pem")
			}

			if tc.breakKeyPem {
				require.NoError(t, os.MkdirAll(filepath.Join(dir, agentKeyName), 0700), "Setup: failed to create a directory that should break key.pem")
			}

			tlsCert, err := certs.CreateTLSCertificateSignedBy(common.AgentCertFilePrefix, "test-server-cn", rootCert, rootKey, dir)

			if tc.wantErr {
				require.Error(t, err, "CreateTLSCertificateSignedBy should have failed")
				return
			}
			require.NoError(t, err, "CreateTLSCertificateSignedBy failed")
			require.NotNil(t, tlsCert, "CreateTLSCertificateSignedBy returned a nil certificate")
			require.FileExists(t, filepath.Join(dir, agentCertName), "CreateTLSCertificateSignedBy failed to write the certificate")
			require.FileExists(t, filepath.Join(dir, agentKeyName), "CreateTLSCertificateSignedBy failed to write the certificate")
		})
	}
}
