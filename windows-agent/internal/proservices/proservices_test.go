package proservices_test

import (
	"context"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	agentapi "github.com/canonical/ubuntu-pro-for-wsl/agentapi/go"
	"github.com/canonical/ubuntu-pro-for-wsl/windows-agent/internal/consts"
	"github.com/canonical/ubuntu-pro-for-wsl/windows-agent/internal/proservices"
	"github.com/canonical/ubuntu-pro-for-wsl/windows-agent/internal/proservices/registrywatcher/registry"
	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func TestMain(m *testing.M) {
	log.SetLevel(log.DebugLevel)

	exit := m.Run()
	defer os.Exit(exit)
}

func TestNew(t *testing.T) {
	t.Parallel()

	testCases := map[string]struct {
		breakConfig      bool
		breakNewDistroDB bool

		wantErr bool
	}{
		"Success when the subscription stays empty":               {},
		"Success when the config cannot check if it is read-only": {breakConfig: true},

		"Error when database cannot create its dump file": {breakNewDistroDB: true, wantErr: true},
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
			reg.CloseKey(k)

			if tc.breakNewDistroDB {
				dbFile := filepath.Join(privateDir, consts.DatabaseFileName)
				err := os.MkdirAll(dbFile, 0600)
				require.NoError(t, err, "Setup: could not write directory where database wants to put a file")
			}

			s, err := proservices.New(ctx, publicDir, privateDir, "", proservices.WithRegistry(reg))
			if err == nil {
				defer s.Stop(ctx)
			}

			if tc.wantErr {
				require.Error(t, err, "New should return an error")
				return
			}
			require.NoError(t, err, "New should return no error")
		})
	}
}

func TestRegisterGRPCServices(t *testing.T) {
	t.Parallel()

	testCases := map[string]struct {
		authToken string

		wantErr bool
	}{
		"Success": {authToken: testToken},

		"Error when requests come without an auth token":   {wantErr: true},
		"Error when requests come with invalid auth token": {authToken: "invalid", wantErr: true},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {

			ctx, cancel := context.WithCancel(context.Background())
			t.Cleanup(cancel)

			ps, err := proservices.New(ctx, t.TempDir(), t.TempDir(), testToken, proservices.WithRegistry(registry.NewMock()))
			require.NoError(t, err, "Setup: New should return no error")
			defer ps.Stop(ctx)

			server := ps.RegisterGRPCServices(context.Background())
			info := server.GetServiceInfo()

			_, ok := info["agentapi.UI"]
			require.True(t, ok, "UI service should be registered after calling RegisterGRPCServices")

			_, ok = info["agentapi.WSLInstance"]
			require.True(t, ok, "WSLInstance service should be registered after calling RegisterGRPCServices")

			require.Lenf(t, info, 2, "Info should contain exactly two elements")

			// Run the server configured by RegisterGRPCServices.
			var cfg net.ListenConfig
			lis, err := cfg.Listen(ctx, "tcp", "localhost:0")
			require.NoError(t, err, "Setup: could not create a listener")
			defer lis.Close()

			go func() {
				err := server.Serve(lis)
				if err != nil {
					t.Logf("Serve exited with error: %v", err)
				}
			}()
			defer server.Stop()

			// Create a client connection to the server.
			addr := lis.Addr().String()
			opts := []grpc.DialOption{grpc.WithTransportCredentials(insecure.NewCredentials())}
			if tc.authToken != "" {
				opts = append(opts, grpc.WithPerRPCCredentials(creds{token: tc.authToken}))
			}
			conn, err := grpc.Dial(addr, opts...)
			require.NoError(t, err, "Setup: could not create a client connection")
			defer conn.Close()
			c := agentapi.NewUIClient(conn)

			// Test the client connection.
			ctx, cancel = context.WithTimeout(context.Background(), 500*time.Millisecond)
			defer cancel()
			_, err = c.Ping(ctx, &agentapi.Empty{})

			if tc.wantErr {
				require.Error(t, err, "Ping should return an error")
				return
			}
			require.NoError(t, err, "Ping should return no error")
		})
	}
}

const testToken = "test-secret-token"

// creds implements the credentials.PerRPCCredentials interface for testing purposes.
type creds struct {
	token string
}

func (a creds) GetRequestMetadata(ctx context.Context, uri ...string) (map[string]string, error) {
	return map[string]string{"authorization": a.token}, nil
}

func (a creds) RequireTransportSecurity() bool {
	return false
}
