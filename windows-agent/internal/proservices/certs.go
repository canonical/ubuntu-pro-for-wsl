package proservices

import (
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"math/big"

	"github.com/canonical/ubuntu-pro-for-wsl/common"
	"github.com/canonical/ubuntu-pro-for-wsl/common/certs"
	"github.com/ubuntu/decorate"
)

// NewTLSCertificates creates a root CA and a server self-signed certificates and writes them into destDir.
func NewTLSCertificates(destDir string) (c Certs, err error) {
	decorate.OnError(&err, "could not create TLS credentials:")

	// generates a pseudo-random serial number for the root CA certificate.
	serial, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return Certs{}, fmt.Errorf("failed to generate serial number for the CA cert: %v", err)
	}

	rootCert, rootKey, err := certs.CreateRootCA(common.GRPCServerNameOverride, serial, destDir)
	if err != nil {
		return Certs{}, err
	}

	// Create and write the server and client certificates signed by the root certificate created above.
	serverCert, err := certs.CreateTLSCertificateSignedBy("server", common.GRPCServerNameOverride, serial.Rsh(serial, 2), rootCert, rootKey, destDir)
	if err != nil {
		return Certs{}, err
	}
	// We won't store the TLS client certificate, because only the agent should access this function and it's not interested in the client TLS certificate.
	// But we still need to write them to disk, so clients can construct their TLS configs from there.
	_, err = certs.CreateTLSCertificateSignedBy("client", common.GRPCServerNameOverride, serial.Lsh(serial, 3), rootCert, rootKey, destDir)
	if err != nil {
		return Certs{}, err
	}

	return Certs{RootCA: rootCert, ServerCert: *serverCert}, nil
}

// ServerTLSConfig returns a TLS config for servers that require and verify client certificates.
func (c Certs) ServerTLSConfig() *tls.Config {
	ca := x509.NewCertPool()
	ca.AddCert(c.RootCA)
	return &tls.Config{
		Certificates: []tls.Certificate{c.ServerCert},
		ClientCAs:    ca,
		ClientAuth:   tls.RequireAndVerifyClientCert,
		MinVersion:   tls.VersionTLS13,
	}
}

// Certs conveniently holds the root CA and server certificates to make it easy to create a TLS config.
type Certs struct {
	RootCA     *x509.Certificate
	ServerCert tls.Certificate
}
