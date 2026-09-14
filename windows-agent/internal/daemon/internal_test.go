package daemon

import (
	"context"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/canonical/ubuntu-pro-for-wsl/common"
	"github.com/canonical/ubuntu-pro-for-wsl/windows-agent/internal/daemon/daemontestutils"
	"github.com/canonical/ubuntu-pro-for-wsl/windows-agent/internal/daemon/testdata/grpctestservice"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
)

const defaultTestTimeout = 5 * time.Second

func waitForGeneration(t *testing.T, registered <-chan int, expectedGen int) {
	t.Helper()
	select {
	case gen := <-registered:
		require.Equal(t, expectedGen, gen, "Expected server generation")
	case <-time.After(defaultTestTimeout):
		require.Failf(t, "Timeout", "Timed out waiting for gRPC server generation %d", expectedGen)
	}
}

func requireServeNoError(t *testing.T, serveErr <-chan error) {
	t.Helper()
	select {
	case err := <-serveErr:
		if err != nil && strings.Contains(err.Error(), grpc.ErrServerStopped.Error()) {
			err = nil
		}
		require.NoError(t, err, "Serve should exit without error after Quit")
	case <-time.After(defaultTestTimeout):
		require.Fail(t, "Serve did not exit in time")
	}
}

// TestRestart verifies the daemon's restart behavior across different lifecycle states:
// - successful multi-generation restarts while serving;
// - no-op when restarting before serving starts;
// - no-op when restarting after the daemon has quit;
// - no-op and no request enqueueing when the context is already cancelled.
func TestRestart(t *testing.T) {
	t.Parallel()

	testsCases := map[string]struct {
		afterQuit     bool
		beforeServing bool
		cancelEarly   bool

		wantAddrFileDeleted bool
	}{
		"Success": {},
		"Does nothing when the context is cancelled":  {cancelEarly: true, wantAddrFileDeleted: true},
		"Does nothing when daemon is not serving yet": {beforeServing: true},
		"Does nothing when the daemon is done":        {afterQuit: true, wantAddrFileDeleted: true},
	}

	for name, tc := range testsCases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			ctx := context.Background()
			addrDir := t.TempDir()

			registered := make(chan int, 10)
			var gen atomic.Int32

			registerer := func(context.Context, bool) *grpc.Server {
				server := grpc.NewServer()
				grpctestservice.RegisterTestServiceServer(server, testGRPCService{})
				registered <- int(gen.Add(1))
				return server
			}

			d := New(ctx, registerer, addrDir)

			if tc.beforeServing {
				done := make(chan struct{})
				go func() {
					d.restart(ctx)
					close(done)
				}()

				select {
				case <-done:
					// proceed.
				case <-time.After(defaultTestTimeout):
					require.Fail(t, "Restart should return immediately when daemon is not serving")
				}

				require.Empty(t, registered, "No server generation should have registered")
			}

			serveErr := make(chan error, 1)
			go func() {
				serveErr <- d.Serve(ctx)
				close(serveErr)
			}()

			addrPath := filepath.Join(addrDir, common.ListeningPortFileName)
			daemontestutils.RequireWaitPathExists(t, addrPath, "Serve should have created an address file")

			// Wait for initial server generation (generation 1)
			waitForGeneration(t, registered, 1)

			if tc.afterQuit {
				d.Quit(ctx, false)
				requireServeNoError(t, serveErr)

				done := make(chan struct{})
				go func() {
					d.restart(ctx)
					close(done)
				}()

				select {
				case <-done:
					// proceed.
				case <-time.After(defaultTestTimeout):
					require.Fail(t, "Restart should return immediately when daemon is stopped")
				}

				require.Empty(t, registered, "No new server generation should have registered after quit")
				daemontestutils.RequireWaitPathDoesNotExist(t, addrPath, "Address file should be removed after Quit")
				return
			}

			if tc.cancelEarly {
				cancelCtx, cancel := context.WithCancel(ctx)
				cancel()

				d.restart(cancelCtx)

				require.Empty(t, d.quit, "d.quit channel should remain empty after cancelled restart")

				d.Quit(ctx, false)
				requireServeNoError(t, serveErr)
				daemontestutils.RequireWaitPathDoesNotExist(t, addrPath, "Address file should be removed after Quit")
				return
			}

			// Normal flow: restart twice and verify generation increases each time
			d.restart(ctx)
			waitForGeneration(t, registered, 2)

			d.restart(ctx)
			waitForGeneration(t, registered, 3)

			d.Quit(ctx, false)
			requireServeNoError(t, serveErr)

			if tc.wantAddrFileDeleted {
				daemontestutils.RequireWaitPathDoesNotExist(t, addrPath, "Address file should be removed after Quit")
			}
		})
	}
}

type testGRPCService struct {
	grpctestservice.UnimplementedTestServiceServer
}
