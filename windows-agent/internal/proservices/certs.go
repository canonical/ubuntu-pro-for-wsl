package proservices

import (
	"crypto/tls"
	"fmt"
	"os"
	"path/filepath"

	"github.com/canonical/ubuntu-pro-for-wsl/common"
	"github.com/canonical/ubuntu-pro-for-wsl/common/certs"
	"github.com/ubuntu/decorate"
)

// newTLSCertificates creates a fresh ephemeral PKI and writes the publishable set
// (root CA certificate, client certificate and client key) into the certificates
// subdirectory of publicDir. The agent's own identity is held in memory and returned
// as a TLS server config; nothing of the agent is ever written to disk.
func newTLSCertificates(publicDir string) (cfg *tls.Config, err error) {
	defer decorate.OnError(&err, "could not create TLS credentials:")

	pki, err := certs.GenerateEphemeralPKI()
	if err != nil {
		return nil, err
	}

	destDir := filepath.Join(publicDir, common.CertificatesDir)
	if err := os.MkdirAll(destDir, 0700); err != nil {
		return nil, fmt.Errorf("failed to create certificates directory: %v", err)
	}

	for _, f := range pki.Publishable {
		if err := os.WriteFile(filepath.Join(destDir, f.Name), f.Bytes, 0600); err != nil {
			return nil, fmt.Errorf("failed to write %s: %v", f.Name, err)
		}
	}

	return pki.AgentTLSConfig, nil
}
