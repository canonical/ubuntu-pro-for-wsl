package endtoend_test

import (
	"bytes"
	"context"
	"crypto/tls"
	"fmt"
	"log"
	"log/slog"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	landscapeapi "github.com/canonical/landscape-hostagent-api"
	"github.com/canonical/ubuntu-pro-for-windows/common/testutils"
	"github.com/canonical/ubuntu-pro-for-windows/mocks/landscape/landscapemockservice"
	"github.com/stretchr/testify/require"
	"github.com/ubuntu/gowsl"
	"golang.org/x/exp/maps"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

// landscape manages all the aspects necessary to hosting a Landscape mock server.
type landscape struct {
	lis     net.Listener
	server  *grpc.Server
	service *landscapemockservice.Service

	// ClientConfig is the configuration file needed to connect to this mock Landscape server.
	ClientConfig string

	logs bytes.Buffer
	stop func()
}

// NewLandscape sets up the Landscape mock server, but does not start it.
//
//nolint:revive // Context goes after testing.T
func NewLandscape(t *testing.T, ctx context.Context) (l landscape) {
	t.Helper()

	ctx, cancel := context.WithCancel(ctx)
	l.stop = cancel
	t.Cleanup(cancel)

	certPath := t.TempDir()
	testutils.GenerateTempCertificate(t, certPath)

	certificatePath := filepath.Join(certPath, "cert.pem")
	privateKeyPath := filepath.Join(certPath, "key.pem")

	serverCert, err := tls.LoadX509KeyPair(certificatePath, privateKeyPath)
	require.NoError(t, err, "Setup: could not load Landscape mock server credentials")

	var cfg net.ListenConfig
	lis, err := cfg.Listen(ctx, "tcp", "localhost:")
	require.NoError(t, err, "Setup: can't listen")
	l.lis = lis

	config := &tls.Config{
		Certificates: []tls.Certificate{serverCert},
		ClientAuth:   tls.NoClientCert,
		MinVersion:   tls.VersionTLS12,
	}

	h := slog.NewTextHandler(&l.logs, &slog.HandlerOptions{Level: slog.LevelDebug})
	l.service = landscapemockservice.New(landscapemockservice.WithLogger(slog.New(h)))

	l.server = grpc.NewServer(grpc.Creds(credentials.NewTLS(config)))
	landscapeapi.RegisterLandscapeHostAgentServer(l.server, l.service)

	l.ClientConfig = fmt.Sprintf(`
	[host]
	url = %s
	
	[client]
	ssl_public_key = %s
	`, lis.Addr(), certificatePath)

	return l
}

// Serve runs the GRPC service, and logs the exit status. This is a blocking call.
func (l *landscape) Serve() {
	if err := l.server.Serve(l.lis); err != nil {
		log.Printf("Landscape server exited with an error: %v", err)
	}
}

// LogOnError prints the Landscape mock server's logs if the test has failed.
func (l landscape) LogOnError(t *testing.T) {
	t.Helper()

	if !t.Failed() {
		return
	}

	t.Logf("Landscape logs:\n%s", l.logs.String())
}

// Stop the Landscape server.
func (l landscape) Stop() {
	if l.stop != nil {
		l.stop()
	}
}

// RequireReceivedInfo checks that a connection to Landscape was made and the proper information was sent.
func (l landscape) RequireReceivedInfo(t *testing.T, wantToken string, wantDistro gowsl.Distro) landscapemockservice.HostInfo {
	t.Helper()

	require.Eventually(t, func() bool {
		return len(l.service.Hosts()) > 0
	}, time.Minute, time.Second, "Landscape should have had at least one connection")

	require.Len(t, l.service.Hosts(), 1, "Landscape should have had only one connection")
	info := maps.Values(l.service.Hosts())[0]

	// Validate token
	require.Equal(t, wantToken, info.Token, "Landscape did not receive the right pro token")

	// Validate distro
	require.Len(t, info.Instances, 1, "Landscape did not receive the right number of distros")
	require.Equal(t, wantDistro.Name(), info.Instances[0].ID, "Landscape did not receive the right distro name from the agent")

	// Validate hostname
	hostname, err := os.Hostname()
	require.NoError(t, err, "could not test machine's hostname")
	require.Equal(t, hostname, info.Hostname, "Landscape did not receive the right hostname from the agent")

	return info
}

// RequireUninstallCommand checks that Landscape can successfully send an uninstall command and that it can be executed.
//
//nolint:revive // testing.T must precede the context
func (l landscape) RequireUninstallCommand(t *testing.T, ctx context.Context, d gowsl.Distro, info landscapemockservice.HostInfo) {
	t.Helper()

	// Validate a Landscape command
	err := l.service.SendCommand(ctx,
		info.UID,
		&landscapeapi.Command{
			Cmd: &landscapeapi.Command_Uninstall_{
				Uninstall: &landscapeapi.Command_Uninstall{
					Id: d.Name(),
				},
			},
		})
	require.NoError(t, err, "could not send an uninstall command via Landscape")
	require.Eventually(t, func() bool {
		reg, err := d.IsRegistered()
		if err != nil {
			t.Logf("While waiting for Landscape uninstall command to complete: %v", err)
		}
		return !reg
	}, time.Minute, time.Second, "Landcape uninstall command never took effect")
}
