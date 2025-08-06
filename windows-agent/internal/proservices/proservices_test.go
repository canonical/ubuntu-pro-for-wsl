package proservices_test

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	agentapi "github.com/canonical/ubuntu-pro-for-wsl/agentapi/go"
	"github.com/canonical/ubuntu-pro-for-wsl/common"
	"github.com/canonical/ubuntu-pro-for-wsl/common/testutils"
	"github.com/canonical/ubuntu-pro-for-wsl/windows-agent/internal/consts"
	"github.com/canonical/ubuntu-pro-for-wsl/windows-agent/internal/proservices"
	"github.com/canonical/ubuntu-pro-for-wsl/windows-agent/internal/proservices/registrywatcher/registry"
	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
)

func TestMain(m *testing.M) {
	log.SetLevel(log.DebugLevel)

	m.Run()
}

func TestNew(t *testing.T) {
	t.Parallel()

	testCases := map[string]struct {
		breakConfig          bool
		breakNewDistroDB     bool
		breakCertificatesDir bool
		breakCA              bool
		breakCloudInit       bool

		wantErr bool
	}{
		"When the subscription stays empty":               {},
		"When the config cannot check if it is read-only": {breakConfig: true},

		"Error when database cannot create its dump file":     {breakNewDistroDB: true, wantErr: true},
		"Error when certificates directory cannot be created": {breakCertificatesDir: true, wantErr: true},
		"Error when CA certificate cannot be created":         {breakCA: true, wantErr: true},
		"Error when cloud-init dir cannot be created":         {breakCloudInit: true, wantErr: true},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			ctx := context.Background()

			publicDir := t.TempDir()
			privateDir := t.TempDir()

			reg := registry.NewMock()
			k, err := reg.HKCUCreateKey("Software/Canonical/UbuntuPro")
			require.NoError(t, err, "Setup: could not create Ubuntu Pro registry key")
			defer reg.CloseKey(k)

			if tc.breakNewDistroDB {
				dbFile := filepath.Join(privateDir, consts.DatabaseFileName)
				err := os.MkdirAll(dbFile, 0600)
				require.NoError(t, err, "Setup: could not write directory where database wants to put a file")
			}
			if tc.breakCertificatesDir {
				require.NoError(t, os.WriteFile(filepath.Join(publicDir, common.CertificatesDir), []byte{}, 0600), "Setup: could not create the file that should break writing the certificates")
			}
			if tc.breakCA {
				require.NoError(t, os.MkdirAll(filepath.Join(publicDir, common.CertificatesDir, common.RootCACertFileName), 0700), "Setup: could not break the root CA certificate file")
			}

			if tc.breakCloudInit {
				f, err := os.Create(filepath.Join(publicDir, ".cloud-init"))
				require.NoError(t, err, "Setup: could not write the file that replaces cloud-init data directory")
				f.Close()
			}

			s, err := proservices.New(ctx, publicDir, privateDir, proservices.WithRegistry(reg))
			if err == nil {
				defer s.Stop(ctx)
			}
			if tc.wantErr {
				require.Error(t, err, "New should return an error")
				return
			}
			require.NoError(t, err, "New should return no error")

			err = reg.WriteValue(k, "LandscapeConfig", "[host]\nurl=lds.company.com:6554\n[client]\nuser=JohnDoe", true)
			require.NoError(t, err, "Setup: could not write LandscapeConfig to the registry mock")
			err = reg.WriteValue(k, "UbuntuProToken", "test-token", false)
			require.NoError(t, err, "Setup: could not write UbuntuProToken to the registry mock")

			agentYamlPath := filepath.Join(publicDir, ".cloud-init", "agent.yaml")

			// Wait for the agent.yaml to be written
			require.Eventually(t, checkFileExists(agentYamlPath), 5*time.Second, 200*time.Millisecond, "agent.yaml file should have been created with registry data")

			got, err := os.ReadFile(filepath.Join(publicDir, ".cloud-init", "agent.yaml"))
			require.NoError(t, err, "Setup: could not read agent.yaml file post test completion")
			want := testutils.LoadWithUpdateFromGolden(t, string(got))
			require.Equal(t, want, string(got), "agent.yaml file should be the same as the golden file")
		})
	}
}

func checkFileExists(path string) func() bool {
	return func() bool {
		s, err := os.Stat(path)
		return err == nil && !s.IsDir()
	}
}

func TestRegisterGRPCServices(t *testing.T) {
	t.Parallel()

	defaultServices := []string{"agentapi.UI", "agentapi.WSLInstance"}

	testCases := map[string]struct {
		insecureClient bool
		withoutWSLNet  bool

		wantServices []string
		wantErr      bool
	}{
		"Success with WSL net adapter":    {wantServices: defaultServices},
		"Success without WSL net adapter": {withoutWSLNet: true, wantServices: []string{"agentapi.UI"}},

		"Error with insecure requests": {insecureClient: true, wantServices: defaultServices, wantErr: true},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			ctx, cancel := context.WithCancel(context.Background())
			t.Cleanup(cancel)

			publicDir := t.TempDir()

			s, err := proservices.New(ctx, publicDir, t.TempDir(), proservices.WithRegistry(registry.NewMock()))
			require.NoError(t, err, "Setup: New should return no error")
			defer s.Stop(ctx)

			server := s.RegisterGRPCServices(context.Background(), !tc.withoutWSLNet)
			info := server.GetServiceInfo()

			for _, service := range tc.wantServices {
				_, ok := info[service]
				require.True(t, ok, "%s service should be registered after calling RegisterGRPCServices", service)
			}

			require.Lenf(t, info, len(tc.wantServices), "Info should contain exactly two elements")

			// Run the server configured by RegisterGRPCServices.
			var cfg net.ListenConfig
			lis, err := cfg.Listen(ctx, "tcp", "localhost:0")
			require.NoError(t, err, "Setup: could not create a listener")
			defer lis.Close()

			serverDone := make(chan struct{})
			go func() {
				defer close(serverDone)
				err := server.Serve(lis)
				if err != nil {
					t.Logf("Serve exited with error: %v", err)
				}
			}()
			t.Cleanup(func() {
				server.Stop()
				<-serverDone
			})

			// Create a client connection to the server.
			addr := lis.Addr().String()
			creds := insecure.NewCredentials()
			if !tc.insecureClient {
				creds = loadClientCertificates(t, filepath.Join(publicDir, common.CertificatesDir))
			}
			conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(creds))
			require.NoError(t, err, "Setup: could not create a client connection")
			defer conn.Close()
			conn.Connect()
			c := agentapi.NewUIClient(conn)

			// Test the client connection.
			ctx, cancel = context.WithTimeout(context.Background(), 500*time.Millisecond)
			defer cancel()
			_, err = c.Ping(ctx, &agentapi.Empty{})

			if tc.wantErr {
				require.Error(t, err, "Clients should fail to call any RPC")
				return
			}
			require.NoError(t, err, "Clients should succeed in calling any RPC")
		})
	}
}

func loadClientCertificates(t *testing.T, certsDir string) credentials.TransportCredentials {
	t.Helper()

	cert, err := tls.LoadX509KeyPair(filepath.Join(certsDir, common.ClientsCertFilePrefix+common.CertificateSuffix), filepath.Join(certsDir, common.ClientsCertFilePrefix+common.KeySuffix))
	require.NoError(t, err, "failed to load client cert: %v", err)

	ca := x509.NewCertPool()
	caFilePath := filepath.Join(certsDir, common.RootCACertFileName)
	caBytes, err := os.ReadFile(caFilePath)
	require.NoError(t, err, "failed to read ca cert %q: %v", caFilePath, err)

	require.True(t, ca.AppendCertsFromPEM(caBytes), "failed to parse %q", caFilePath)

	tlsConfig := &tls.Config{
		MinVersion:   tls.VersionTLS13,
		ServerName:   common.GRPCServerNameOverride,
		Certificates: []tls.Certificate{cert},
		RootCAs:      ca,
	}

	return credentials.NewTLS(tlsConfig)
}
