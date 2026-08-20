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

// newTLSCertificatesOpts holds the configurable dependencies for newTLSCertificates.
// It exists primarily so tests can inject failures without mutating global state.
type newTLSCertificatesOpts struct {
	generatePKI func() (certs.PKI, error)
}

// newTLSCertificatesOption configures newTLSCertificatesOpts.
type newTLSCertificatesOption func(*newTLSCertificatesOpts)

// newTLSCertificates creates a fresh ephemeral PKI and writes the publishable set
// (root CA certificate, client certificate and client key) into the certificates
// subdirectory of publicDir. Any stale files left over from a previous run or a
// previous code version are removed first. The agent's own identity is held in
// memory and returned as a TLS server config; nothing of the agent is ever
// written to disk.
func newTLSCertificates(publicDir string, opts ...newTLSCertificatesOption) (cfg *tls.Config, err error) {
	defer decorate.OnError(&err, "could not create TLS credentials:")

	options := newTLSCertificatesOpts{
		generatePKI: certs.GenerateEphemeralPKI,
	}
	for _, o := range opts {
		o(&options)
	}

	pki, err := options.generatePKI()
	if err != nil {
		return nil, err
	}

	destDir := filepath.Join(publicDir, common.CertificatesDir)

	// Clean any stale certificate material from a previous run. We only remove
	// the directory contents, not the directory itself, so that the
	// "certificates dir cannot be created" error path (a file exists at the
	// destination path) is preserved.
	if info, err := os.Stat(destDir); err == nil && info.IsDir() {
		entries, err := os.ReadDir(destDir)
		if err != nil {
			return nil, fmt.Errorf("failed to read certificates directory: %v", err)
		}
		for _, entry := range entries {
			// Leave directories untouched: a directory occupying the path of a
			// file we need to write is a legitimate error condition we preserve.
			if entry.IsDir() {
				continue
			}
			if err := os.Remove(filepath.Join(destDir, entry.Name())); err != nil {
				return nil, fmt.Errorf("failed to clean certificates directory: %v", err)
			}
		}
	}

	if err := os.MkdirAll(destDir, 0700); err != nil {
		return nil, fmt.Errorf("failed to create certificates directory: %v", err)
	}

	for name, data := range pki.PEMFiles {
		if err := os.WriteFile(filepath.Join(destDir, name), data, 0600); err != nil {
			return nil, fmt.Errorf("failed to write %s: %v", name, err)
		}
	}

	return pki.AgentTLSConfig, nil
}
