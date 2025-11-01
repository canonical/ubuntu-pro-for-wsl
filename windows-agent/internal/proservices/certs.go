package proservices

import (
	"crypto/tls"
	"crypto/x509"

	"github.com/canonical/ubuntu-pro-for-wsl/common"
	"github.com/canonical/ubuntu-pro-for-wsl/common/certs"
	"github.com/ubuntu/decorate"
)

// newTLSCertificates creates a self-signed root CA, agent and clients certificates and writes them into destDir.
func newTLSCertificates(destDir string) (c agentCerts, err error) {
	decorate.OnError(&err, "could not create TLS credentials:")

	rootCert, rootKey, err := certs.CreateRootCA(common.GRPCServerNameOverride, destDir)
	if err != nil {
		return agentCerts{}, err
	}

	// Create and write the agent and clients certificates signed by the root certificate created above.
	agentCert, err := certs.CreateTLSCertificateSignedBy(common.AgentCertFilePrefix, common.GRPCServerNameOverride, rootCert, rootKey, destDir)
	if err != nil {
		return agentCerts{}, err
	}
	// We won't store the TLS client certificate, because only the agent should access this function and it's not interested in the client TLS certificate.
	// But we still need to write them to disk, so clients can construct their TLS configs from there.
	_, err = certs.CreateTLSCertificateSignedBy(common.ClientsCertFilePrefix, common.GRPCServerNameOverride, rootCert, rootKey, destDir)
	if err != nil {
		return agentCerts{}, err
	}

	return agentCerts{rootCA: rootCert, agentCert: *agentCert}, nil
}

// agentTLSConfig returns a TLS config for the agent that require and verify client certificates.
func (c agentCerts) agentTLSConfig() *tls.Config {
	ca := x509.NewCertPool()
	ca.AddCert(c.rootCA)
	return &tls.Config{
		Certificates: []tls.Certificate{c.agentCert},
		ClientCAs:    ca,
		ClientAuth:   tls.RequireAndVerifyClientCert,
		MinVersion:   tls.VersionTLS13,
	}
}

// agentCerts conveniently holds the root CA and the agent certificates to make it easy to create a TLS config.
type agentCerts struct {
	rootCA    *x509.Certificate
	agentCert tls.Certificate
}
