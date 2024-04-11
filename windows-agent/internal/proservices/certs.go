// Package proservices is in charge of managing the GRPC services and all business-logic side.
package proservices

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"path/filepath"

	"github.com/canonical/ubuntu-pro-for-wsl/common"
	"github.com/ubuntu/decorate"
)

type Certs struct {
	RootCA     *x509.Certificate
	ServerCert tls.Certificate
}

func (c Certs) ServerTLSConfig() *tls.Config {
	ca := x509.NewCertPool()
	ca.AddCert(c.RootCA)
	return &tls.Config{
		Certificates: []tls.Certificate{c.ServerCert},
		ClientCAs:    ca,
		ClientAuth:   tls.RequireAndVerifyClientCert,
	}
}

// NewTLSCertificates creates a root CA and a server self-signed certificates and writes them into destDir.
func NewTLSCertificates(destDir string) (certs Certs, err error) {
	decorate.OnError(&err, "could not create TLS credentials:")

	// generate a new key-pair for the root certificate based on the P256 elliptic curve
	rootKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return Certs{}, fmt.Errorf("failed to generate random key: %v", err)
	}

	rootCertTmpl, err := common.CertTemplate(common.GRPCServerNameOverride)
	if err != nil {
		return Certs{}, fmt.Errorf("failed to create root cert template: %v", err)
	}

	// this cert will be the CA that we will use to sign the server cert
	rootCertTmpl.IsCA = true
	rootCertTmpl.Subject.CommonName = "UP4W CA"
	rootCertTmpl.KeyUsage = x509.KeyUsageCertSign

	rootCert, rootDER, err := common.CreateCert(rootCertTmpl, rootCertTmpl, &rootKey.PublicKey, rootKey)
	if err != nil {
		return Certs{}, err
	}

	// Write the CA certificate to disk.
	if err = common.WriteCert(filepath.Join(destDir, "ca_cert.pem"), rootDER); err != nil {
		return Certs{}, err
	}

	// Create and write the server and client certificates signed by the root certificate created above.
	serverCert, err := common.CreateRootSignedTLSCertificate("server", common.GRPCServerNameOverride, rootCert, rootKey, destDir)
	if err != nil {
		return Certs{}, err
	}
	// We won't store the TLS client certificate, because only the agent should acess this function and it's not interested in the client TLS certificate.
	// But we still need to write them to disk, so clients can construct their TLS configs from there.
	_, err = common.CreateRootSignedTLSCertificate("client", common.GRPCServerNameOverride, rootCert, rootKey, destDir)
	if err != nil {
		return Certs{}, err
	}

	return Certs{RootCA: rootCert, ServerCert: *serverCert}, nil
}
