package daemon_test

import (
	"context"
	"errors"
	"io/fs"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/canonical/ubuntu-pro-for-wsl/common"
	"github.com/canonical/ubuntu-pro-for-wsl/windows-agent/internal/daemon"
	"github.com/canonical/ubuntu-pro-for-wsl/windows-agent/internal/daemon/daemontestutils"
	"github.com/canonical/ubuntu-pro-for-wsl/windows-agent/internal/daemon/netmonitoring"
	"github.com/canonical/ubuntu-pro-for-wsl/windows-agent/internal/daemon/testdata/grpctestservice"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/connectivity"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
)

func init() {
	// Ensures we use the networking-related mocks in all daemon tests unless otherwise locally specified.
	daemontestutils.DefaultNetworkDetectionToMock()
}
func TestNew(t *testing.T) {
	t.Parallel()

	var regCount int
	countRegistrations := func(context.Context, bool) *grpc.Server {
		regCount++
		return nil
	}

	_ = daemon.New(context.Background(), countRegistrations, t.TempDir())
	require.Equal(t, 0, regCount, "daemon should not register GRPC services before serving")
}

func TestStartQuit(t *testing.T) {
	t.Parallel()

	testsCases := map[string]struct {
		forceQuit           bool
		preexistingPortFile bool
		cancelEarly         bool

		wantConnectionsDropped bool
	}{
		"Graceful quit":                              {},
		"Graceful quit, overwrite port file":         {preexistingPortFile: true},
		"Forceful quit":                              {forceQuit: true, wantConnectionsDropped: true},
		"Does nothing when the context is cancelled": {cancelEarly: true, wantConnectionsDropped: true},
	}

	for name, tc := range testsCases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			addrDir := t.TempDir()

			if tc.preexistingPortFile {
				err := os.MkdirAll(addrDir, 0600)
				require.NoError(t, err, "Setup: failed to create pre-exisiting cache directory")
				err = os.WriteFile(filepath.Join(addrDir, common.ListeningPortFileName), []byte("# Old port file"), 0600)
				require.NoError(t, err, "Setup: failed to create pre-existing port file")
			}

			registerer := func(context.Context, bool) *grpc.Server {
				server := grpc.NewServer()
				var service testGRPCService
				grpctestservice.RegisterTestServiceServer(server, service)
				return server
			}

			d := daemon.New(ctx, registerer, addrDir)

			serveErr := make(chan error)
			go func() {
				serveErr <- d.Serve(ctx)
				close(serveErr)
			}()

			addrPath := filepath.Join(addrDir, common.ListeningPortFileName)

			var addrContents []byte
			var err error

			if tc.preexistingPortFile {
				require.Eventually(t, func() bool {
					addrContents, err = os.ReadFile(addrPath)
					require.NoError(t, err, "Address file should be readable")
					return string(addrContents) != "# Old port file"
				}, 5*time.Second, 100*time.Millisecond, "Pre-existing address file should be overwritten after dameon.New()")
			} else {
				requireWaitPathExists(t, addrPath, "Serve should create an address file")
				addrContents, err = os.ReadFile(addrPath)
				require.NoError(t, err, "Address file should be readable")
			}

			// Now we know the TCP server has started.

			address := string(addrContents)
			t.Logf("Address is %q", address)

			_, port, err := net.SplitHostPort(address)
			_, err = net.LookupPort("tcp4", port)
			require.NoError(t, err, "Port should be valid")

			// We start a connection but don't close it yet, so as to test graceful vs. forceful Quit
			closeHangingConn := grpcPersistentCall(t, address)
			defer closeHangingConn()

			// Now we know the GRPC server has started serving.

			if tc.cancelEarly {
				cancel()
				require.Error(t, <-serveErr, "Serve should return with error when stopped by the context")
			}
			// Handle Quit firing
			serverStopped := make(chan struct{})
			go func() {
				d.Quit(ctx, tc.forceQuit)
				close(serverStopped)
			}()

			var immediateQuit bool
			select {
			case <-serverStopped:
				immediateQuit = true
			case <-time.After(time.Second):
			}

			if tc.wantConnectionsDropped {
				require.True(t, immediateQuit, "Force quit should quit immediately regardless of exisiting connections")

				code := closeHangingConn()
				require.Equal(t, codes.Unavailable, code, "GRPC call should return an error of type %q, instead got %q", codes.Unavailable, code)
			} else {
				// We have an hanging connection which should make us time out
				require.False(t, immediateQuit, "Quit should wait for existing connections to close before quitting")
				requireCannotDialGRPC(t, address, "No new connection should be allowed after calling Quit")

				// release hanging connection and wait for Quit to exit.
				code := closeHangingConn()
				require.Equal(t, codes.Canceled, code, "GRPC call should return an error of type %q, instead got %q", codes.Canceled, code)
				<-serverStopped
			}

			require.NoError(t, <-serveErr, "Serve should return no error when stopped normally")
			requireCannotDialGRPC(t, address, "No new connection should be allowed when the server is no longer running")
			requireWaitPathDoesNotExist(t, addrPath, "Address file should have been removed after quitting the server")
		})
	}
}

func TestRestart(t *testing.T) {
	t.Parallel()

	testsCases := map[string]struct {
		afterQuit     bool
		beforeServing bool
		cancelEarly   bool

		wantAddrFileDeleted bool
		wantServeErr        bool
	}{
		"Success": {},
		"Does nothing when the context is cancelled":  {cancelEarly: true, wantAddrFileDeleted: true, wantServeErr: true},
		"Does nothing when daemon is not serving yet": {beforeServing: true},
		"Does nothing when the daemon is done":        {afterQuit: true, wantAddrFileDeleted: true},
	}

	for name, tc := range testsCases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			addrDir := t.TempDir()

			registerer := func(context.Context, bool) *grpc.Server {
				server := grpc.NewServer()
				var service testGRPCService
				grpctestservice.RegisterTestServiceServer(server, service)
				return server
			}

			d := daemon.New(ctx, registerer, addrDir)

			serveErr := make(chan error, 1)

			if tc.beforeServing {
				go func() {
					d.Restart(ctx)
					serveErr <- nil
				}()

				select {
				case <-time.After(100 * time.Millisecond):
					require.Fail(t, "Restart should return immediately when daemon is not serving")
				case <-serveErr:
					// proceed.
				}
			}

			go func() {
				serveErr <- d.Serve(ctx)
				close(serveErr)
			}()

			addrPath := filepath.Join(addrDir, common.ListeningPortFileName)

			var err error
			requireWaitPathExists(t, addrPath, "Serve should have created a .address file")
			addrSt, err := os.Stat(addrPath)
			require.NoError(t, err, "Address file should be readable")

			if tc.afterQuit {
				d.Quit(ctx, false)
			}
			if tc.cancelEarly {
				cancel()
			}
			// Now we know the GRPC server has started serving.
			d.Restart(ctx)

			// d.Serve() shouldn't have exitted with an error yet at this point.
			select {
			case err := <-serveErr:
				if tc.wantServeErr {
					require.Error(t, err, "Serve should return with error when stopped by the context")
				} else {
					require.NoError(t, err, "Restart should not have caused Serve() to exit with an error")
				}
			case <-time.After(100 * time.Millisecond):
				// proceed.
			}

			if tc.wantAddrFileDeleted {
				requireWaitPathDoesNotExist(t, addrPath, "Address file should have been removed after quitting the server")
				return
			}

			requireWaitPathExists(t, addrPath, "Restart should have caused creation of another .address file")
			// Contents could be the same without our control, thus best to check the file time.
			newAddrSt, err := os.Stat(addrPath)
			require.NoError(t, err, "Address file should be readable")
			require.NotEqual(t, addrSt.ModTime(), newAddrSt.ModTime(), "Address file should be overwritten after Restart")

			// Restart a second time
			d.Restart(ctx)
			// d.Serve() shouldn't have exitted with an error yet at this point.
			select {
			case err := <-serveErr:
				require.NoError(t, err, "Restart should not have caused Serve() to exit with an error")
			case <-time.After(100 * time.Millisecond):
				// proceed.
			}
		})
	}
}

func TestServeWSLIP(t *testing.T) {
	t.Parallel()

	registerer := func(context.Context, bool) *grpc.Server {
		return grpc.NewServer()
	}

	testcases := map[string]struct {
		netmode      string
		withAdapters daemontestutils.MockIPAdaptersState
		subscribeErr error

		wantErr bool
	}{
		"Success":                       {withAdapters: daemontestutils.MultipleHyperVAdaptersInList},
		"With a single Hyper-V Adapter": {withAdapters: daemontestutils.SingleHyperVAdapterInList},
		"With mirrored networking mode": {netmode: "mirrored", withAdapters: daemontestutils.MultipleHyperVAdaptersInList},
		"With no access to the system distro but net mode is the default (NAT)": {netmode: "error", withAdapters: daemontestutils.MultipleHyperVAdaptersInList},

		"When the networking mode is unknown":            {netmode: "unknown"},
		"Wwhen the list of adapters is empty":            {withAdapters: daemontestutils.EmptyList},
		"When listing adapters requires too much memory": {withAdapters: daemontestutils.RequiresTooMuchMem},
		"When there is no Hyper-V adapter the list":      {withAdapters: daemontestutils.NoHyperVAdapterInList},
		"When retrieving adapters information fails":     {withAdapters: daemontestutils.MockError},

		"Error when the WSL IP cannot be found and monitoring network fails": {withAdapters: daemontestutils.NoHyperVAdapterInList, subscribeErr: errors.New("mock error"), wantErr: true},
	}

	for name, tc := range testcases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			addrDir := t.TempDir()
			// Very lenient timeout because we either expect Serve to fail immediately or we stop it manually.
			// As the last resource, the test will fail due to the context timeout (otherwise it would hang indefinitely).
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
			defer cancel()

			d := daemon.New(ctx, registerer, addrDir)
			defer d.Quit(ctx, false)

			if tc.netmode == "" {
				tc.netmode = "nat"
			}
			mock := daemontestutils.NewHostIPConfigMock(tc.withAdapters)

			serveErr := make(chan error)
			go func() {
				serveErr <- d.Serve(ctx, daemon.WithWslNetworkingMode(tc.netmode), daemon.WithMockedGetAdapterAddresses(mock),
					daemon.WithNetDevicesAPIProvider(
						func() (netmonitoring.DevicesAPI, error) {
							if tc.subscribeErr != nil {
								return nil, tc.subscribeErr
							}
							return &daemontestutils.NetMonitoringMockAPI{}, nil
						},
					))
				close(serveErr)
			}()

			if tc.wantErr {
				require.Error(t, <-serveErr, "Serve should fail when the WSL IP cannot be found")
				return
			}

			serverStopped := make(chan struct{})
			go func() {
				time.Sleep(500 * time.Millisecond)
				d.Quit(ctx, false)
				close(serverStopped)
			}()
			<-serverStopped

			err := <-serveErr
			if err != nil && strings.Contains(err.Error(), grpc.ErrServerStopped.Error()) {
				// We stopped the server manually, so we expect this error, although it's possible that there is not even an error at this point.
				err = nil
			}
			require.NoError(t, err, "Serve should return no error when stopped normally")

			select {
			case <-ctx.Done():
				// Most likely, Serve did not fail and instead started serving,
				// only to be stopped by the test timeout.
				require.Fail(t, "Serve should have failed immediately")
			default:
			}
		})
	}
}

// TestAddingWSLAdapterRestarts simulates the appearance of the WSL adapter after the daemon is running.
func TestAddingWSLAdapterRestarts(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	addrDir := t.TempDir()

	registerer := func(context.Context, bool) *grpc.Server {
		server := grpc.NewServer()
		var service testGRPCService
		grpctestservice.RegisterTestServiceServer(server, service)
		return server
	}

	d := daemon.New(ctx, registerer, addrDir)

	systemNotification := make(chan error)
	defer close(systemNotification)

	mock := daemontestutils.NewHostIPConfigMock(daemontestutils.NoHyperVAdapterInList)

	serveErr := make(chan error, 1)
	go func() {
		serveErr <- d.Serve(ctx, daemon.WithMockedGetAdapterAddresses(mock),
			daemon.WithNetDevicesAPIProvider(daemontestutils.NetDevicesMockAPIWithAddedWSL(systemNotification)),
		)
		close(serveErr)
	}()

	addrPath := filepath.Join(addrDir, common.ListeningPortFileName)

	requireWaitPathExists(t, addrPath, "Serve should create an address file")
	addrSt, err := os.Stat(addrPath)
	require.NoError(t, err, "Address file should be readable")

	// Now we know the GRPC server has started serving. Let's emulate the OS triggering a notification.
	systemNotification <- nil

	// d.Serve() shouldn't have exitted with an error yet at this point.
	select {
	case err := <-serveErr:
		require.NoError(t, err, "Restart should not have caused Serve() to exit with an error")
	case <-time.After(200 * time.Millisecond):
		// proceed.
	}

	requireWaitPathExists(t, addrPath, "Restart should have caused creation of another .address file")
	// Contents could be the same without our control, thus best to check the file time.
	newAddrSt, err := os.Stat(addrPath)
	require.NoError(t, err, "Address file should be readable")
	require.NotEqual(t, addrSt.ModTime(), newAddrSt.ModTime(), "Address file should be overwritten after Restart")
}

func TestServeError(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	addrDir := t.TempDir()

	registerer := func(context.Context, bool) *grpc.Server {
		return grpc.NewServer()
	}

	d := daemon.New(ctx, registerer, addrDir)
	defer d.Quit(ctx, false)

	// Remove parent directory to prevent listening port file to be written
	require.NoError(t, os.RemoveAll(addrDir), "Setup: could not remove cache directory")

	err := d.Serve(ctx)
	require.Error(t, err, "Serve should fail when the cache dir does not exist")
}

func TestQuitBeforeServe(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	addrDir := t.TempDir()

	registerer := func(context.Context, bool) *grpc.Server {
		return grpc.NewServer()
	}

	d := daemon.New(ctx, registerer, addrDir)
	d.Quit(ctx, false)

	serverErr := make(chan error)
	go func() {
		defer close(serverErr)
		serverErr <- d.Serve(ctx)
	}()

	<-time.After(100 * time.Millisecond)
	d.Quit(ctx, false)

	require.NoError(t, <-serverErr, "Calling Serve() after Quit() should not result in an error")

	requireWaitPathDoesNotExist(t, filepath.Join(addrDir, common.ListeningPortFileName), "Port file should not exist after returning from Serve()")
}

// grpcPersistentCall will create a persistent GRPC connection to the server.
// It will return immediately. drop() should be called to ends the connection from
// the client side. It returns the GRPC error code if any.
func grpcPersistentCall(t *testing.T, addr string) (drop func() codes.Code) {
	t.Helper()

	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	require.NoErrorf(t, err, "Could not create a GRPC client.")

	c := grpctestservice.NewTestServiceClient(conn)
	ctx, cancel := context.WithCancel(context.Background())

	started := make(chan struct{})
	errch := make(chan error)
	go func() {
		close(started)
		_, err = c.Blocking(ctx, new(grpctestservice.Empty))
		errch <- err
		close(errch)
	}()

	<-started
	// Wait for the call being initiated.
	time.Sleep(100 * time.Millisecond)

	return func() codes.Code {
		// Give some slack for the client if we aborted the server.
		time.Sleep(time.Millisecond * 100)
		cancel()
		err, ok := <-errch
		if !ok {
			return codes.OK
		}
		// Transform the GRPC error to go errors
		st, grpcErr := status.FromError(err)
		require.True(t, grpcErr, "Unexpected error type from GRPC call: %v", err)
		return st.Code()
	}
}

// requireCannotDialGRPC attempts to.
func requireCannotDialGRPC(t *testing.T, addr string, msg string) {
	t.Helper()

	// Try to connect. Non-blocking call so no error is wanted.
	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	require.NoErrorf(t, err, "error dialing GRPC server.\nMessage: %s", msg)
	defer conn.Close()
	conn.Connect()

	// Timing out and checking that the connection was never established.
	time.Sleep(300 * time.Millisecond)
	validStates := []connectivity.State{connectivity.Connecting, connectivity.TransientFailure}
	require.Contains(t, validStates, conn.GetState(), "unexpected state after dialing. Expected any of %q but got %q", validStates, conn.GetState())
}

// requireWaitPathExists checks periodically for the existence of a path. If the path
// does not exist after waiting for the specified timeout, the test fails. This function
// is blocking.
func requireWaitPathExists(t *testing.T, path string, msg string) {
	t.Helper()

	fileExists := func() bool {
		_, err := os.Lstat(path)
		if err == nil {
			return true
		}
		require.ErrorIsf(t, err, fs.ErrNotExist, "could not stat path %q. Message: %s", path, msg)
		return false
	}

	require.Eventually(t, fileExists, 5*time.Second, 100*time.Millisecond, "%q does not exists: %v", path, msg)

	// Prevent error when accessing the file right after:
	// 'The process cannot access the file because it is being used by another process'
	time.Sleep(10 * time.Millisecond)
}

// requireWaitPathDoesNotExist checks periodically for the existence of a path. If the path
// does not exist after waiting for the specified timeout, the test fails. This function
// is blocking.athDoesNotExist checks periodiclly for the existence of a path. If the path
// does not exist after waiting for the specified timeout, the test fails. This function
// is blocking.
func requireWaitPathDoesNotExist(t *testing.T, path string, msg string) {
	t.Helper()

	var err error
	fileDoesNotExist := func() bool {
		_, err = os.Lstat(path)
		if err == nil {
			return false
		}
		require.ErrorIsf(t, err, fs.ErrNotExist, "could not stat path %q. Message: %s", path, msg)
		return true
	}

	require.Eventually(t, fileDoesNotExist, 100*time.Millisecond, time.Millisecond, "%q still exists: %v", path, msg)
}

// Our mock GRPC service.
type testGRPCService struct {
	grpctestservice.UnimplementedTestServiceServer
}

func (testGRPCService) Blocking(ctx context.Context, e *grpctestservice.Empty) (*grpctestservice.Empty, error) {
	<-ctx.Done()
	return &grpctestservice.Empty{}, nil
}

func TestWithWslSystemMock(t *testing.T) { daemontestutils.MockWslSystemCmd(t) }
