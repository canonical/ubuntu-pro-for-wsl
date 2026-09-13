package proservices_test

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	agentapi "github.com/canonical/ubuntu-pro-for-wsl/agentapi/go"
	"github.com/canonical/ubuntu-pro-for-wsl/common"
	"github.com/canonical/ubuntu-pro-for-wsl/common/certs"
	grpclog "github.com/canonical/ubuntu-pro-for-wsl/common/grpc/logstreamer"
	"github.com/canonical/ubuntu-pro-for-wsl/common/testutils"
	"github.com/canonical/ubuntu-pro-for-wsl/common/wsltestutils"
	"github.com/canonical/ubuntu-pro-for-wsl/windows-agent/internal/consts"
	"github.com/canonical/ubuntu-pro-for-wsl/windows-agent/internal/proservices"
	"github.com/canonical/ubuntu-pro-for-wsl/windows-agent/internal/proservices/registrywatcher/registry"
	"github.com/canonical/ubuntu-pro-for-wsl/windows-agent/internal/securefiles"
	"github.com/canonical/ubuntu-pro-for-wsl/windows-agent/internal/securefiles/securefilestest"
	log "github.com/sirupsen/logrus"
	"github.com/sirupsen/logrus/hooks/test"
	"github.com/stretchr/testify/require"
	wsl "github.com/ubuntu/gowsl"
	wslmock "github.com/ubuntu/gowsl/mock"
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

	hook := test.NewGlobal()
	t.Cleanup(hook.Reset)

	testCases := map[string]struct {
		breakConfig      bool
		breakNewDistroDB bool
		breakCloudInit   bool

		breakCertsDir     bool
		breakCertificates bool

		breakLandscapeConnect bool
		staleDistroData       bool

		wantErr         bool
		wantErrContains string
	}{
		"When the subscription stays empty":               {},
		"When the config cannot check if it is read-only": {breakConfig: true},

		"Error when database cannot create its dump file": {breakNewDistroDB: true, wantErr: true},
		"Error when cloud-init dir cannot be created":     {breakCloudInit: true, wantErr: true},
		"Error when the certificates dir is a file":       {breakCertsDir: true, wantErr: true, wantErrContains: "failed to open certificates directory"},
		"Error when the certificates dir is not writable": {breakCertificates: true, wantErr: true, wantErrContains: "failed to create certificates"},

		"Warning when the first Landscape connection fails":               {breakLandscapeConnect: true},
		"Warning when a stale distro's cloud-init data cannot be removed": {staleDistroData: true},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			ctx := t.Context()

			if (tc.breakCertificates || tc.staleDistroData) && (runtime.GOOS == "windows" || os.Geteuid() == 0) {
				t.Skip("read-only directory semantics require a non-root Unix user")
			}

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

			if tc.breakCloudInit {
				f, err := os.Create(filepath.Join(publicDir, ".cloud-init"))
				require.NoError(t, err, "Setup: could not write the file that replaces cloud-init data directory")
				f.Close()
			}

			if tc.breakCertsDir {
				f, err := os.Create(filepath.Join(publicDir, common.CertificatesDir))
				require.NoError(t, err, "Setup: could not write the file that replaces the certificates directory")
				f.Close()
			}

			if tc.breakCertificates {
				// A read-only certificates directory opens fine but rejects the fresh PKI material.
				certsDir := filepath.Join(publicDir, common.CertificatesDir)
				require.NoError(t, os.MkdirAll(certsDir, 0700), "Setup: could not create the certificates directory")
				//nolint:gosec // G302 - test setup removes directory write permission.
				require.NoError(t, os.Chmod(certsDir, 0500), "Setup: could not make the certificates directory read-only")
				//nolint:gosec // G302 - test teardown restores directory permissions.
				t.Cleanup(func() { _ = os.Chmod(certsDir, 0700) })
			}

			if tc.breakLandscapeConnect {
				// A Landscape config valid enough to be accepted, but pointing at a
				// certificate that does not exist: the first connection attempt fails.
				require.NoError(t, reg.WriteValue(k, "UbuntuProToken", "test-token", false), "Setup: could not write UbuntuProToken to the registry mock")
				badCert := filepath.Join(t.TempDir(), "nonexistent.pem")
				require.NoError(t, reg.WriteValue(k, "LandscapeConfig",
					"[host]\nurl=localhost:6554\n[client]\nssl_public_key="+badCert, true),
					"Setup: could not write LandscapeConfig to the registry mock")
			}

			if tc.staleDistroData {
				// A distro that becomes stale after startup whose cloud-init data cannot
				// be removed: the cleanup callback must only warn. The database holds the
				// currently-registered name/GUID pair so it loads; re-registering the
				// distro with a fresh GUID before Stop makes it invalid, so the cleanup
				// during Stop fires the callback. The callback receives the lower-cased
				// database key.
				if wsl.MockAvailable() {
					ctx = wsl.WithMock(ctx, wslmock.New())
				}
				const staleName = "testDistro_UP4W_StaleDistro"
				guid := wsltestutils.RegisterDistroNamed(t, ctx, staleName)
				dbContent := fmt.Sprintf(`- name: %s
  guid: '%s'
  properties:
    distroid: %s
    versionid: "22.04"
    prettyname: Stale Distro
    proattached: false
    hostname: StaleHost
`, staleName, guid, staleName)
				require.NoError(t, os.WriteFile(filepath.Join(privateDir, consts.DatabaseFileName), []byte(dbContent), 0600),
					"Setup: could not seed the database with the stale distro")

				keepDir := filepath.Join(publicDir, ".cloud-init", strings.ToLower(staleName)+".user-data", "keep")
				require.NoError(t, os.MkdirAll(keepDir, 0700), "Setup: could not create obstructing user-data directory")
				require.NoError(t, os.WriteFile(filepath.Join(keepDir, "keep.txt"), []byte("x"), 0600), "Setup: could not fill obstructing user-data directory")
				//nolint:gosec // G302 - test setup removes directory write permission.
				require.NoError(t, os.Chmod(keepDir, 0500), "Setup: could not make obstructing directory read-only")
				//nolint:gosec // G302 - test teardown restores directory permissions.
				t.Cleanup(func() { _ = os.Chmod(keepDir, 0700) })

				t.Cleanup(func() { wsltestutils.UnregisterDistro(t, ctx, staleName) })
			}

			publicCustodian, err := securefiles.Open(publicDir)
			require.NoError(t, err, "Setup: could not open public custodian")
			defer publicCustodian.Close()

			s, err := proservices.New(ctx, publicCustodian, privateDir, proservices.WithRegistry(reg))
			if err == nil {
				defer s.Stop(ctx)
			}
			if tc.wantErr {
				require.Error(t, err, "New should return an error")
				if tc.wantErrContains != "" {
					require.Contains(t, err.Error(), tc.wantErrContains)
				}
				return
			}
			require.NoError(t, err, "New should return no error")

			if tc.breakLandscapeConnect {
				require.Contains(t, hookMessages(hook), "could not connect to Landscape server",
					"A failing first Landscape connection must warn but not fail")
				return
			}

			if tc.staleDistroData {
				// Re-registering with a fresh GUID makes the database entry stale, so the
				// cleanup during Stop fires the removal callback on the obstructed data.
				wsltestutils.ReregisterDistro(t, ctx, "testDistro_UP4W_StaleDistro", false)
				s.Stop(ctx)

				require.Contains(t, hookMessages(hook), "Could not remove leftover distro data",
					"A failing cleanup callback must warn but not fail")
				require.DirExists(t, filepath.Join(publicDir, ".cloud-init", "testdistro_up4w_staledistro.user-data"),
					"The unremovable cloud-init data should have been left in place")
				return
			}

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

// hookMessages joins all messages collected by a logrus test hook for
// substring assertions.
func hookMessages(hook *test.Hook) string {
	var msgs []string
	for _, e := range hook.AllEntries() {
		msgs = append(msgs, e.Message)
	}
	return strings.Join(msgs, "\n")
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

			publicCustodian, err := securefiles.Open(publicDir)
			require.NoError(t, err, "Setup: could not open public custodian")
			defer publicCustodian.Close()

			s, err := proservices.New(ctx, publicCustodian, t.TempDir(), proservices.WithRegistry(registry.NewMock()))
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

// TestOnNewInstanceCreatesTask verifies that when a distro connects to the WSLInstance
// gRPC service for the first time, the onNewInstance hook registered by the proservices
// Manager submits a ProAttachment task whose token matches the one stored in the registry.
func TestOnNewInstanceCreatesTask(t *testing.T) {
	if wsl.MockAvailable() {
		t.Parallel()
	}

	testcases := map[string]struct {
		alreadyAttached bool
		breakConfig     bool
		wantCommands    bool
	}{
		"When the instance is not already attached, onNewInstance should submit a ProAttachment task": {wantCommands: true},
		"When the instance is already attached, onNewInstance should not submit a ProAttachment task": {alreadyAttached: true},
		"When the subscription cannot be retrieved":                                                   {breakConfig: true},
	}

	for name, tc := range testcases {
		t.Run(name, func(t *testing.T) {
			ctx := t.Context()

			if wsl.MockAvailable() {
				ctx = wsl.WithMock(ctx, wslmock.New())
				t.Parallel()
			}

			publicDir := t.TempDir()
			privateDir := t.TempDir()

			// Populate the registry with a Pro token *before* New() so that the registry watcher's
			// initial read picks it up and the config contains the subscription when the distro connects.
			reg := registry.NewMock()
			k, err := reg.HKCUCreateKey("Software/Canonical/UbuntuPro")
			require.NoError(t, err, "Setup: could not create Ubuntu Pro registry key")
			defer reg.CloseKey(k)

			publicCustodian, err := securefiles.Open(publicDir)
			require.NoError(t, err, "Setup: could not open public custodian")
			defer publicCustodian.Close()

			s, err := proservices.New(ctx, publicCustodian, privateDir, proservices.WithRegistry(reg))
			require.NoError(t, err, "Setup: New should return no error")
			defer s.Stop(ctx)

			const wantToken = "test-pro-token"
			if tc.breakConfig {
				path := filepath.Join(privateDir, "config")
				require.NoError(t, os.Remove(path), "Setup: could not remove the config file")
				require.NoError(t, os.Mkdir(path, 0640), "Setup: could not break the config file")
			} else {
				err = reg.WriteValue(k, "UbuntuProToken", wantToken, false)
				require.NoError(t, err, "Setup: could not write UbuntuProToken to the registry mock")
			}

			server := s.RegisterGRPCServices(ctx, true)

			var cfg net.ListenConfig
			lis, err := cfg.Listen(ctx, "tcp", "localhost:0")
			require.NoError(t, err, "Setup: could not create a listener")
			defer lis.Close()

			serverDone := make(chan struct{})
			go func() {
				defer close(serverDone)
				_ = server.Serve(lis)
			}()
			defer func() {
				server.Stop()
				<-serverDone
			}()

			// Register a fake distro so the DB's Get/Create path succeeds (requires a real WSL entry).
			distroName, _ := wsltestutils.RegisterDistro(t, ctx, false)

			// Dial the gRPC server using TLS credentials identical to those used in TestRegisterGRPCServices.
			creds := loadClientCertificates(t, filepath.Join(publicDir, common.CertificatesDir))
			conn, err := grpc.NewClient(lis.Addr().String(),
				grpc.WithTransportCredentials(creds),
				grpc.WithStreamInterceptor(grpclog.StreamClientInterceptor(log.StandardLogger())),
			)
			require.NoError(t, err, "Setup: could not create a gRPC client connection")
			defer conn.Close()
			conn.Connect()

			wslClient := agentapi.NewWSLInstanceClient(conn)

			// Open all three streams that the server requires before making the connection "ready",
			// effectively mocking the wsl-pro-service unit inside an instance.
			connStream, err := wslClient.Connected(ctx)
			require.NoError(t, err, "Setup: could not open Connected stream")

			proStream, err := wslClient.ProAttachmentCommands(ctx)
			require.NoError(t, err, "Setup: could not open ProAttachmentCommands stream")

			lpeStream, err := wslClient.LandscapeConfigCommands(ctx)
			require.NoError(t, err, "Setup: could not open LandscapeConfigCommands stream")

			// Perform the handshakes. Order must match what the wslinstance server expects: first send
			// DistroInfo on the Connected stream, then send the WSL name on the two command streams.
			err = connStream.Send(&agentapi.DistroInfo{WslName: distroName, ProAttached: tc.alreadyAttached})
			require.NoError(t, err, "Setup: could not send DistroInfo on Connected stream")

			sendWslNameMsg := func(send func(*agentapi.MSG) error) {
				t.Helper()
				require.NoError(t, send(&agentapi.MSG{Data: &agentapi.MSG_WslName{WslName: distroName}}),
					"Setup: could not send WSL name on command stream")
			}
			sendWslNameMsg(proStream.Send)
			sendWslNameMsg(lpeStream.Send)

			// The server sends a ProAttachCmd once all streams are ready and onNewInstance runs.
			// Receive it with a generous timeout to avoid flakiness.
			type recvResult struct {
				cmd *agentapi.ProAttachCmd
				err error
			}
			recvCh := make(chan recvResult, 1)
			go func() {
				cmd, err := proStream.Recv()
				recvCh <- recvResult{cmd, err}
			}()

			const timeout = 20 * time.Second
			select {
			case result := <-recvCh:
				if tc.wantCommands {
					require.NoError(t, result.err, "ProAttachmentCommands stream should have received a command")
					require.Equal(t, wantToken, result.cmd.GetToken(), "ProAttachCmd token should match the registry-provided token")
				} else {
					require.Error(t, result.err, "ProAttachmentCommands stream should not have received any commands")
				}
			case <-time.After(timeout):
				if tc.wantCommands {
					t.Fatal("Timed out waiting for ProAttachCmd from the server")
				}
			}

			if tc.wantCommands {
				// Acknowledge the command so the server's SendProAttachment call completes cleanly.
				require.NoError(t, proStream.Send(&agentapi.MSG{Data: &agentapi.MSG_Result{Result: ""}}),
					"Could not acknowledge ProAttachCmd")
			}
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
		MinVersion:   certs.MinTLSVersion,
		ServerName:   common.GRPCServerNameOverride,
		Certificates: []tls.Certificate{cert},
		RootCAs:      ca,
	}

	return credentials.NewTLS(tlsConfig)
}

// TestNewReplacesPreExistingForeignSubtrees verifies that pre-existing first-level
// subtree roots (.cloud-init and certificates) are replaced rather than adopted
// when proservices.New starts.
func TestNewReplacesPreExistingForeignSubtrees(t *testing.T) {
	publicDir := t.TempDir()
	privateDir := t.TempDir()

	// Pre-create foreign unstamped directories
	cloudInitDir := filepath.Join(publicDir, ".cloud-init")
	certsDir := filepath.Join(publicDir, common.CertificatesDir)
	require.NoError(t, os.MkdirAll(cloudInitDir, 0700))
	require.NoError(t, os.MkdirAll(certsDir, 0700))

	// Seed foreign files
	require.NoError(t, os.WriteFile(filepath.Join(cloudInitDir, "foreign.txt"), []byte("foreign"), 0600))
	require.NoError(t, os.WriteFile(filepath.Join(certsDir, "stale.key"), []byte("stale"), 0600))

	publicCustodian, err := securefiles.Open(publicDir)
	require.NoError(t, err, "Setup: could not open public custodian")
	defer publicCustodian.Close()

	s, err := proservices.New(context.Background(), publicCustodian, privateDir, proservices.WithRegistry(registry.NewMock()))
	require.NoError(t, err, "Setup: New should return no error")
	defer s.Stop(context.Background())

	// The certificates directory has a fixed set of legitimate filenames, so stale
	// content is removed by name on every platform.
	require.NoFileExists(t, filepath.Join(certsDir, "stale.key"), "stale certificates file should be removed")

	// Cloud-init uses watermark ownership, so the raw foreign file is purged here too.
	require.NoFileExists(t, filepath.Join(cloudInitDir, "foreign.txt"), "foreign cloud-init file should be removed")

	if runtime.GOOS == "windows" {
		// Verify the directory nodes themselves are stamped root-owned 0700.
		uid, gid, mode, err := securefilestest.ReadLxAttributes(cloudInitDir)
		require.NoError(t, err, "could not read .cloud-init extended attributes")
		require.Equal(t, uint32(0), uid, ".cloud-init should be owned by root")
		require.Equal(t, uint32(0), gid, ".cloud-init should be group root")
		require.Equal(t, uint32(040700), mode, ".cloud-init should have mode 0700")

		uid, gid, mode, err = securefilestest.ReadLxAttributes(certsDir)
		require.NoError(t, err, "could not read certificates extended attributes")
		require.Equal(t, uint32(0), uid, "certificates directory should be owned by root")
		require.Equal(t, uint32(0), gid, "certificates directory should be group root")
		require.Equal(t, uint32(040700), mode, "certificates directory should have mode 0700")
	}
}
