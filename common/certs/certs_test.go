package certs_test

import (
	"crypto/tls"
	"crypto/x509"
	"testing"

	"github.com/canonical/ubuntu-pro-for-wsl/common"
	"github.com/canonical/ubuntu-pro-for-wsl/common/certs"
	"github.com/stretchr/testify/require"
)

func TestGenerateEphemeralPKI(t *testing.T) {
	t.Parallel()

	pki, err := certs.GenerateEphemeralPKI()
	require.NoError(t, err, "GenerateEphemeralPKI failed")
	require.NotNil(t, pki.AgentTLSConfig, "AgentTLSConfig should not be nil")

	// Verify publishable set contains exactly the three expected files.
	expectedFiles := map[string]struct{}{
		common.RootCACertFileName:                               {},
		common.ClientsCertFilePrefix + common.CertificateSuffix: {},
		common.ClientsCertFilePrefix + common.KeySuffix:         {},
	}

	require.Len(t, pki.PEMFiles, len(expectedFiles), "PEMFiles should contain exactly %d files", len(expectedFiles))
	for name := range expectedFiles {
		require.Contains(t, pki.PEMFiles, name, "PEMFiles should contain %s", name)
		require.NotEmpty(t, pki.PEMFiles[name], "PEM file %s should not be empty", name)
	}
}

func TestHandshake(t *testing.T) {
	t.Parallel()

	pki1, err := certs.GenerateEphemeralPKI()
	require.NoError(t, err)

	pki2, err := certs.GenerateEphemeralPKI()
	require.NoError(t, err)

	clientCert1, err := tls.X509KeyPair(
		pki1.PEMFiles[common.ClientsCertFilePrefix+common.CertificateSuffix],
		pki1.PEMFiles[common.ClientsCertFilePrefix+common.KeySuffix],
	)
	require.NoError(t, err)

	caPool1 := x509.NewCertPool()
	require.True(t, caPool1.AppendCertsFromPEM(pki1.PEMFiles[common.RootCACertFileName]))

	clientTLSConfig1 := &tls.Config{
		Certificates: []tls.Certificate{clientCert1},
		RootCAs:      caPool1,
		ServerName:   common.GRPCServerNameOverride,
		MinVersion:   certs.MinTLSVersion,
	}

	clientCert2, err := tls.X509KeyPair(
		pki2.PEMFiles[common.ClientsCertFilePrefix+common.CertificateSuffix],
		pki2.PEMFiles[common.ClientsCertFilePrefix+common.KeySuffix],
	)
	require.NoError(t, err)

	caPool2 := x509.NewCertPool()
	require.True(t, caPool2.AppendCertsFromPEM(pki2.PEMFiles[common.RootCACertFileName]))

	clientTLSConfig2 := &tls.Config{
		Certificates: []tls.Certificate{clientCert2},
		RootCAs:      caPool2,
		ServerName:   common.GRPCServerNameOverride,
		MinVersion:   certs.MinTLSVersion,
	}

	// Start listener with pki1's AgentTLSConfig
	listener, err := tls.Listen("tcp", "127.0.0.1:0", pki1.AgentTLSConfig)
	require.NoError(t, err)
	defer listener.Close()

	done := make(chan error, 1)
	go func() {
		conn, err := listener.Accept()
		if err != nil {
			done <- err
			return
		}
		defer conn.Close()
		tlsConn, ok := conn.(*tls.Conn)
		if !ok {
			done <- nil
			return
		}
		done <- tlsConn.Handshake()
	}()

	// Connect with matching client (pki1) -> success
	conn, err := tls.Dial("tcp", listener.Addr().String(), clientTLSConfig1)
	require.NoError(t, err)
	require.NoError(t, conn.Handshake())
	conn.Close()

	serverErr := <-done
	require.NoError(t, serverErr)

	// Now try connecting with a client from a different PKI (pki2) -> rejected
	go func() {
		conn, err := listener.Accept()
		if err != nil {
			done <- err
			return
		}
		defer conn.Close()
		tlsConn, ok := conn.(*tls.Conn)
		if !ok {
			done <- nil
			return
		}
		done <- tlsConn.Handshake()
	}()

	conn2, err := tls.Dial("tcp", listener.Addr().String(), clientTLSConfig2)
	if err == nil {
		err = conn2.Handshake()
		conn2.Close()
		require.Error(t, err, "Client from different PKI should fail handshake")
	} else {
		require.Error(t, err, "Dial with different PKI client should fail")
	}

	<-done // accept routine finished
}
