package daemon_test

import (
	"context"
	"io/fs"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/canonical/ubuntu-pro-for-wsl/common"
	"github.com/canonical/ubuntu-pro-for-wsl/windows-agent/internal/daemon"
	"github.com/canonical/ubuntu-pro-for-wsl/windows-agent/internal/daemon/testdata/grpctestservice"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/connectivity"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
)

func TestNew(t *testing.T) {
	t.Parallel()

	var regCount int
	countRegistrations := func(context.Context) *grpc.Server {
		regCount++
		return nil
	}

	_ = daemon.New(context.Background(), countRegistrations, t.TempDir())
	require.Equal(t, 1, regCount, "daemon should register GRPC services only once")
}

func TestStartQuit(t *testing.T) {
	t.Parallel()

	testsCases := map[string]struct {
		forceQuit           bool
		preexistingPortFile bool

		wantConnectionsDropped bool
	}{
		"Graceful quit":                      {},
		"Graceful quit, overwrite port file": {preexistingPortFile: true},
		"Forceful quit":                      {forceQuit: true, wantConnectionsDropped: true},
	}

	for name, tc := range testsCases {
		tc := tc
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			ctx := context.Background()
			addrDir := t.TempDir()

			if tc.preexistingPortFile {
				err := os.MkdirAll(addrDir, 0600)
				require.NoError(t, err, "Setup: failed to create pre-exisiting cache directory")
				err = os.WriteFile(filepath.Join(addrDir, common.ListeningPortFileName), []byte("# Old port file"), 0600)
				require.NoError(t, err, "Setup: failed to create pre-existing port file")
			}

			registerer := func(context.Context) *grpc.Server {
				server := grpc.NewServer()
				var service testGRPCService
				grpctestservice.RegisterTestServiceServer(server, service)
				return server
			}

			d := daemon.New(ctx, registerer, addrDir)

			serveErr := make(chan error)
			go func() {
				serveErr <- d.Serve(ctx)
			}()

			addrPath := filepath.Join(addrDir, common.ListeningPortFileName)

			var addrContents []byte
			var err error

			if tc.preexistingPortFile {
				require.Eventually(t, func() bool {
					addrContents, err = os.ReadFile(addrPath)
					require.NoError(t, err, "Address file should be readable")
					return string(addrContents) != "# Old port file"
				}, 500*time.Millisecond, 50*time.Millisecond, "Pre-existing address file should be overwritten after dameon.New()")
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
				require.False(t, immediateQuit, "Quit should wait for exisiting connections to close before quitting")
				requireCannotDialGRPC(t, address, "No new connection should be allowed after calling Quit")

				// release hanging connection and wait for Quit to exit.
				code := closeHangingConn()
				require.Equal(t, codes.Canceled, code, "GRPC call should return an error of type %q, instead got %q", codes.Canceled, code)
				<-serverStopped
			}

			require.NoError(t, <-serveErr, "Serve should return no error when stopped normally")
			requireCannotDialGRPC(t, address, "No new connection should be allowed when the server is no longer running")
			requireWaitPathDoesNotExist(t, addrPath, "Address file should be removed after quitting the server")
		})
	}
}

func TestServeError(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	addrDir := t.TempDir()

	registerer := func(context.Context) *grpc.Server {
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

	registerer := func(context.Context) *grpc.Server {
		return grpc.NewServer()
	}

	d := daemon.New(ctx, registerer, addrDir)
	d.Quit(ctx, false)

	err := d.Serve(ctx)
	require.Error(t, err, "Calling Serve() after Quit() should result in an error")

	requireWaitPathDoesNotExist(t, filepath.Join(addrDir, common.ListeningPortFileName), "Port file should not exist after returning from Serve()")
}

// grpcPersistentCall will create a persistent GRPC connection to the server.
// It will return immediately. drop() should be called to ends the connection from
// the client side. It returns the GRPC error code if any.
func grpcPersistentCall(t *testing.T, addr string) (drop func() codes.Code) {
	t.Helper()

	const timeout = 100 * time.Millisecond
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	conn, err := grpc.DialContext(ctx, addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	require.NoErrorf(t, err, "Could not dial GRPC server.")

	c := grpctestservice.NewTestServiceClient(conn)
	ctx, cancel = context.WithCancel(context.Background())

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
	conn, err := grpc.Dial(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	require.NoErrorf(t, err, "error dialing GRPC server.\nMessage: %s", msg)
	defer conn.Close()

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

	require.Eventually(t, fileExists, 500*time.Millisecond, 50*time.Millisecond, "%q does not exists: %v", path, msg)

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
