//go:build windows

package daemon_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/canonical/ubuntu-pro-for-wsl/common"
	"github.com/canonical/ubuntu-pro-for-wsl/windows-agent/internal/daemon"
	"github.com/canonical/ubuntu-pro-for-wsl/windows-agent/internal/securefiles"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
)

// TestHeldAddressFileDoesNotStopTheDaemon pins that a holder of the previous address
// file node — which the custodian refuses to unlink, answering ErrNodeHeld — never
// takes the daemon down: the publication is retried briefly, then the daemon serves
// on, and the held node keeps its content for whatever still reads it. Windows-only
// because the refusal itself is: POSIX cannot refuse an unlink because a descriptor
// is open, so the condition does not exist to be tested elsewhere. The classification
// of the refusal is pinned on the securefiles side (TestCreateFileReportsAHeldNode,
// TestCreateFileAnswersAHeldNodeWithTheSentinel).
func TestHeldAddressFileDoesNotStopTheDaemon(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	addrDir := t.TempDir()
	c, err := securefiles.Open(addrDir)
	require.NoError(t, err, "Setup: could not create custodian")
	defer func() { _ = c.Close() }()

	// Seed a previous address file and hold it the way host-side software that
	// does not share deletion does: an ordinary os.Open shares read and write.
	addrPath := filepath.Join(addrDir, common.ListeningPortFileName)
	require.NoError(t, c.WriteFile(common.ListeningPortFileName, []byte("old-address")),
		"Setup: could not seed the address file")
	holder, err := os.Open(addrPath)
	require.NoError(t, err, "Setup: could not hold the address file open")

	registerer := func(context.Context, bool) *grpc.Server {
		return grpc.NewServer()
	}

	d := daemon.New(ctx, registerer, c)

	serveErr := make(chan error, 1)
	go func() {
		serveErr <- d.Serve(ctx)
	}()
	d.WaitReady()

	// The retry budget (250+500+750ms) outlasted: the daemon must be serving
	// anyway, and the held node must stand exactly as it was.
	time.Sleep(2 * time.Second)
	select {
	case err := <-serveErr:
		require.Fail(t, "a held address file must not stop the daemon", err)
	default:
	}
	got, err := os.ReadFile(addrPath)
	require.NoError(t, err, "the held address file must be readable")
	require.Equal(t, "old-address", string(got), "the held node must keep its content")

	// The hold lifted; the daemon still stops cleanly, and the held node is
	// removed by the shutdown path as usual.
	require.NoError(t, holder.Close(), "Setup: could not lift the hold")

	d.Quit(ctx, false)
	err = <-serveErr
	if err != nil && strings.Contains(err.Error(), grpc.ErrServerStopped.Error()) {
		// We stopped the server manually, so we expect this error, although it is possible that there is not even an error at this point.
		err = nil
	}
	require.NoError(t, err, "Serve should return no error when stopped normally")
	require.NoFileExists(t, addrPath, "the address file must be removed on shutdown")
}
