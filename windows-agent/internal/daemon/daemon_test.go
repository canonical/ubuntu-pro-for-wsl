package daemon_test

import (
	"context"
	"io/fs"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/canonical/ubuntu-pro-for-windows/windows-agent/internal/consts"
	"github.com/canonical/ubuntu-pro-for-windows/windows-agent/internal/daemon"
	"github.com/canonical/ubuntu-pro-for-windows/windows-agent/internal/daemon/testdata/grpctestservice"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/connectivity"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
)

func TestNew(t *testing.T) {
	t.Parallel()

	testCases := map[string]struct {
		badCacheDir bool

		wantErr bool
	}{
		"happy path":    {},
		"bad cache dir": {badCacheDir: true, wantErr: true},
	}

	for name, tc := range testCases {
		tc := tc
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			cacheDir := filepath.Join(t.TempDir(), "cache")
			if tc.badCacheDir {
				err := os.WriteFile(cacheDir, []byte("this is here to break the daemon"), 0600)
				require.NoError(t, err, "Setup: failed to create broken directory as file")
			}

			var regCount int
			countRegistrations := func(context.Context) *grpc.Server {
				regCount++
				return nil
			}

			_, err := daemon.New(context.Background(), countRegistrations, daemon.WithCacheDir(cacheDir))
			if tc.wantErr {
				require.Error(t, err, "New should have errored out but hasn't")
				return
			}

			require.NoError(t, err, "Unexpected error when calling New")
			require.Equal(t, 1, regCount, "daemon should register GRPC services only once")

			require.DirExists(t, cacheDir, "Cache dir should've been created in New")
		})
	}
}

func TestStartQuit(t *testing.T) {
	t.Parallel()

	testsCases := map[string]struct {
		forceQuit           bool
		preexistingPortFile bool

		wantConnectionsDropped bool
	}{
		"graceful":                      {},
		"graceful, overwrite port file": {preexistingPortFile: true},
		"forceful":                      {forceQuit: true, wantConnectionsDropped: true},
	}

	for name, tc := range testsCases {
		tc := tc
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			ctx := context.Background()
			cacheDir := t.TempDir()

			if tc.preexistingPortFile {
				err := os.MkdirAll(cacheDir, 0600)
				require.NoError(t, err, "Setup: failed to create pre-exisiting cache directory")
				err = os.WriteFile(filepath.Join(cacheDir, consts.ListeningPortFileName), []byte("# Old port file"), 0600)
				require.NoError(t, err, "Setup: failed to create pre-exisiting port file")
			}

			var regCount int
			registerer := func(context.Context) *grpc.Server {
				regCount++
				server := grpc.NewServer()
				var service testGRPCService
				grpctestservice.RegisterTestServiceServer(server, service)
				return server
			}

			d, err := daemon.New(ctx, registerer, daemon.WithCacheDir(cacheDir))
			require.NoError(t, err, "New should return the daemon handler")

			serveErr := make(chan error)
			go func() {
				serveErr <- d.Serve(ctx)
			}()

			addrPath := filepath.Join(cacheDir, consts.ListeningPortFileName)
			var addrContents []byte

			if tc.preexistingPortFile {
				require.Eventually(t, func() bool {
					addrContents, err = os.ReadFile(addrPath)
					require.NoError(t, err, "Could not read address file")
					return string(addrContents) != "# Old port file"
				}, 100*time.Millisecond, 10*time.Millisecond, "Pre-existing port file was never overwritten")
			} else {
				requireWaitPathExists(t, addrPath, "Serve never created an address file")
				addrContents, err = os.ReadFile(addrPath)
				require.NoError(t, err, "Could not read address file")
			}

			// Now we know the TCP server has started.

			address := string(addrContents)
			t.Logf("Address is %q", address)
			require.NotEqual(t, nil, net.ParseIP(address), "Address is not valid")

			// We start a connection but don't close it yet, so as to test graceful vs. forceful Quit
			closeHangingConn := grpcPersistentCall(t, address, "Could not dial daemon")
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
				require.True(t, immediateQuit, "Force quit should have quit immediately regardless of exisiting connections")

				code := closeHangingConn()
				require.Equal(t, codes.Unavailable, code, "Unexpected error in GRPC call: %v", code.String())
			} else {
				// We have an hanging connection which should make us time out
				require.False(t, immediateQuit, "Quit should have waited for exisiting connections to close before quitting")
				requireCannotDialGRPC(t, address, "Server is running, but no new connection should be allowed")

				// release hanging connection and wait for Quit to exit.
				code := closeHangingConn()
				require.Equal(t, codes.Canceled, code, "Unexpected error in GRPC call: %v", code.String())
				<-serverStopped
			}

			require.NoError(t, <-serveErr, "Serve should return no error when stopped normally")
			requireCannotDialGRPC(t, address, "Server is not running, no new connection should be allowed")
			requireWaitPathDoesNotExist(t, addrPath, "Address file is not removed after quitting")

			require.Equal(t, 1, regCount, "daemon should register GRPC services only once")
		})
	}
}

func TestServeError(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	cacheDir := t.TempDir()

	registerer := func(context.Context) *grpc.Server {
		return grpc.NewServer()
	}

	d, err := daemon.New(ctx, registerer, daemon.WithCacheDir(cacheDir))
	require.NoError(t, err, "New should return the daemon handler")
	defer d.Quit(ctx, false)

	// Remove parent directory to prevent listening port file to be written
	require.NoError(t, os.RemoveAll(cacheDir), "Setup: could not remove cache directory")

	err = d.Serve(ctx)
	require.Error(t, err, "Unexpected success serving while we could not write the listening port file")
}

func TestQuitBeforeServe(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	cacheDir := t.TempDir()

	registerer := func(context.Context) *grpc.Server {
		return grpc.NewServer()
	}

	d, err := daemon.New(ctx, registerer, daemon.WithCacheDir(cacheDir))
	require.NoError(t, err, "New should return the daemon handler")

	d.Quit(ctx, false)

	err = d.Serve(ctx)
	require.Error(t, err, "Unexpected success serving after having quit")

	requireWaitPathDoesNotExist(t, filepath.Join(cacheDir, consts.ListeningPortFileName), "Port file exists after failed Serve")
}

// grpcPersistentCall will create a persistent GRPC connection to the server.
// It will return immediately. drop() should be called to ends the connection from
// the client side. It returns the GRPC error code if any.
func grpcPersistentCall(t *testing.T, addr string, msg string) (drop func() codes.Code) {
	t.Helper()

	const timeout = 100 * time.Millisecond

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	conn, err := grpc.DialContext(ctx, addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	require.NoErrorf(t, err, "Could not dial GRPC server.\nMessage: %s", msg)

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

	// Try to connect and return once the connection dropped.
	conn, err := grpc.Dial(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	require.NoErrorf(t, err, "error dialing GRPC server.\nMessage: %s", msg)
	defer conn.Close()

	require.Eventuallyf(t, func() bool {
		return conn.GetState() == connectivity.TransientFailure
	}, 100*time.Millisecond, 10*time.Millisecond, "Should have failed to connect to GRPC server after many connection attempts. Connection state is currently at %v.\nMessage: %s", conn.GetState(), msg)
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

	require.Eventually(t, fileExists, 100*time.Millisecond, time.Millisecond, "%q does not exists: %v", path, msg)
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
