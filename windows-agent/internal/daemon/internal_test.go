package daemon

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/canonical/ubuntu-pro-for-wsl/common"
	"github.com/canonical/ubuntu-pro-for-wsl/windows-agent/internal/daemon/daemontestutils"
	"github.com/canonical/ubuntu-pro-for-wsl/windows-agent/internal/daemon/testdata/grpctestservice"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
)

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
				grpctestservice.RegisterTestServiceServer(server, testGRPCService{})
				return server
			}

			d := New(ctx, registerer, addrDir)

			serveErr := make(chan error)

			if tc.beforeServing {
				go func() {
					d.restart(ctx)
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
			daemontestutils.RequireWaitPathExists(t, addrPath, "Serve should have created a .address file")
			addrSt, err := os.Stat(addrPath)
			require.NoError(t, err, "Address file should be readable")

			if tc.afterQuit {
				d.Quit(ctx, false)
			}
			if tc.cancelEarly {
				cancel()
			}
			// Now we know the GRPC server has started serving.
			d.restart(ctx)

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
				daemontestutils.RequireWaitPathDoesNotExist(t, addrPath, "Address file should have been removed after quitting the server")
				return
			}

			daemontestutils.RequireWaitPathExists(t, addrPath, "Restart should have caused creation of another .address file")
			// Contents could be the same without our control, thus best to check the file time.
			newAddrSt, err := os.Stat(addrPath)
			require.NoError(t, err, "Address file should be readable")
			require.NotEqual(t, addrSt.ModTime(), newAddrSt.ModTime(), "Address file should be overwritten after Restart")

			// Restart a second time
			d.restart(ctx)
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

type testGRPCService struct {
	grpctestservice.UnimplementedTestServiceServer
}
