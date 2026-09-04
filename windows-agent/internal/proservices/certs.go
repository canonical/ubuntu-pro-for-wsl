package proservices

import (
	"crypto/tls"
	"fmt"

	"github.com/canonical/ubuntu-pro-for-wsl/common/certs"
	"github.com/canonical/ubuntu-pro-for-wsl/windows-agent/internal/securefiles"
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
// sub-tree through the securefiles custodian. Any stale files left over from a
// previous run or a previous code version are removed first. The agent's own
// identity is held in memory and returned as a TLS server config; nothing of the
// agent is ever written to disk.
//
// The custodian is owned by the caller; this function must not close it.
func newTLSCertificates(certsDir *securefiles.Custodian, opts ...newTLSCertificatesOption) (cfg *tls.Config, err error) {
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

	// Clean any stale certificate material from a previous run. We only remove
	// the sub-tree contents, not the sub-tree itself, so that a directory sitting
	// at the path of a file we need to write remains a legitimate error condition.
	entries, err := certsDir.ReadDir(".")
	if err != nil {
		return nil, fmt.Errorf("failed to read certificates directory: %v", err)
	}
	for _, entry := range entries {
		// Leave directories untouched: a directory occupying the path of a file we
		// need to write must surface as an error, not be silently removed.
		if entry.IsDir() {
			continue
		}
		if err := certsDir.Remove(entry.Name()); err != nil {
			return nil, fmt.Errorf("failed to clean certificates directory: %v", err)
		}
	}

	for name, data := range pki.PEMFiles {
		if err := certsDir.WriteFile(name, data); err != nil {
			return nil, fmt.Errorf("failed to write %s: %v", name, err)
		}
	}

	return pki.AgentTLSConfig, nil
}
