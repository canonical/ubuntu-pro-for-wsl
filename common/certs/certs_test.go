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

	// Verify publishable set names exactly the three files: ca_cert.pem, client_cert.pem, client_key.pem
	expectedFiles := [3]string{
		common.RootCACertFileName,
		common.ClientsCertFilePrefix + common.CertificateSuffix,
		common.ClientsCertFilePrefix + common.KeySuffix,
	}

	require.Equal(t, expectedFiles[0], pki.Publishable[0].Name, "First publishable file should be CA cert")
	require.Equal(t, expectedFiles[1], pki.Publishable[1].Name, "Second publishable file should be client cert")
	require.Equal(t, expectedFiles[2], pki.Publishable[2].Name, "Third publishable file should be client key")

	for i := range 3 {
		require.NotEmpty(t, pki.Publishable[i].Bytes, "Publishable file %s should not be empty", pki.Publishable[i].Name)
	}
}

func TestHandshake(t *testing.T) {
	t.Parallel()

	pki1, err := certs.GenerateEphemeralPKI()
	require.NoError(t, err)

	pki2, err := certs.GenerateEphemeralPKI()
	require.NoError(t, err)

	clientCert1, err := tls.X509KeyPair(pki1.Publishable[1].Bytes, pki1.Publishable[2].Bytes)
	require.NoError(t, err)

	caPool1 := x509.NewCertPool()
	require.True(t, caPool1.AppendCertsFromPEM(pki1.Publishable[0].Bytes))

	clientTLSConfig1 := &tls.Config{
		Certificates: []tls.Certificate{clientCert1},
		RootCAs:      caPool1,
		ServerName:   common.GRPCServerNameOverride,
		MinVersion:   tls.VersionTLS13,
	}

	clientCert2, err := tls.X509KeyPair(pki2.Publishable[1].Bytes, pki2.Publishable[2].Bytes)
	require.NoError(t, err)

	caPool2 := x509.NewCertPool()
	require.True(t, caPool2.AppendCertsFromPEM(pki2.Publishable[0].Bytes))

	clientTLSConfig2 := &tls.Config{
		Certificates: []tls.Certificate{clientCert2},
		RootCAs:      caPool2,
		ServerName:   common.GRPCServerNameOverride,
		MinVersion:   tls.VersionTLS13,
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
		err = tlsConn.Handshake()
		done <- err
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
		err = tlsConn.Handshake()
		done <- err
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
