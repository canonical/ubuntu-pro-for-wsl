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
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/health"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
)

// TestReleasingTheHoldRestoresConnectivity pins the recovery the previous round's
// hold can otherwise deny: the startup attempt outlasted by the holder leaves the
// address file naming the previous, closed listener — without taking the daemon
// down, the held node keeping its content for whatever still reads it — and
// releasing the hold must make the serving daemon re-publish the current
// listener's address and restore connectivity through it. Windows-only because
// the hold itself is: POSIX cannot refuse an unlink because a descriptor is open.
// The drain is pinned here too: after the round ends and removes the file, no
// late publication may resurrect it. The classification of the refusal is pinned
// on the securefiles side (TestCreateFileReportsAHeldNode,
// TestCreateFileAnswersAHeldNodeWithTheSentinel).
func TestReleasingTheHoldRestoresConnectivity(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	addrDir := t.TempDir()
	c, err := securefiles.Open(addrDir)
	require.NoError(t, err, "Setup: could not create custodian")
	defer func() { _ = c.Close() }()

	addrPath := filepath.Join(addrDir, common.ListeningPortFileName)
	require.NoError(t, c.WriteFile(common.ListeningPortFileName, []byte("old-address")),
		"Setup: could not seed the address file")
	holder, err := os.Open(addrPath)
	require.NoError(t, err, "Setup: could not hold the address file open")

	registerer := func(context.Context, bool) *grpc.Server {
		server := grpc.NewServer()
		healthpb.RegisterHealthServer(server, health.NewServer())
		return server
	}

	d := daemon.New(ctx, registerer, c)

	serveErr := make(chan error, 1)
	go func() {
		serveErr <- d.Serve(ctx, daemon.WithPublicationInterval(100*time.Millisecond))
	}()
	d.WaitReady()

	// The startup budget outlasted: the file must still name the stale address,
	// and the holder's refusal must not have taken the daemon down either.
	time.Sleep(2 * time.Second)
	select {
	case err := <-serveErr:
		require.Fail(t, "a held address file must not stop the daemon", err)
	default:
	}
	got, err := os.ReadFile(addrPath)
	require.NoError(t, err, "the held address file must be readable")
	require.Equal(t, "old-address", string(got), "while held, the file must keep the previous address")

	// The hold lifts; the next tick must republish the current listener.
	require.NoError(t, holder.Close(), "Setup: could not lift the hold")
	addr := ""
	require.Eventually(t, func() bool {
		got, err := os.ReadFile(addrPath)
		if err != nil || string(got) == "old-address" {
			return false
		}
		addr = string(got)
		return true
	}, 5*time.Second, 50*time.Millisecond, "the address file must be republished after the hold lifts")

	// Connectivity, not just content: the freshly published address must name
	// the live listener, reachable with a real gRPC call.
	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	require.NoError(t, err, "the republished address must be dialable")
	defer conn.Close()
	client := healthpb.NewHealthClient(conn)
	resp, err := client.Check(ctx, &healthpb.HealthCheckRequest{})
	require.NoError(t, err, "a gRPC call through the republished address must succeed")
	require.Equal(t, healthpb.HealthCheckResponse_SERVING, resp.GetStatus(), "the daemon must answer through the republished address")

	// The round ends; the drained watchdog must not resurrect the file.
	d.Quit(ctx, false)
	err = <-serveErr
	if err != nil && strings.Contains(err.Error(), grpc.ErrServerStopped.Error()) {
		// We stopped the server manually, so we expect this error, although it is possible that there is not even an error at this point.
		err = nil
	}
	require.NoError(t, err, "Serve should return no error when stopped normally")
	require.NoFileExists(t, addrPath, "the address file must be removed on shutdown")

	time.Sleep(500 * time.Millisecond)
	require.NoFileExists(t, addrPath, "no late publication may resurrect the address file")
}

// TestServeErrorDrainsThePublicationWatchdog pins that the watchdog cannot
// outlive the round it serves: a serving goroutine that exits on its own —
// here, a gRPC server stopped before Serve — ends the round through the errCh
// branch of tryServingOnce, and the stop callback that branch owes is what
// cancels and drains the watchdog. Without it, the watchdog survives the
// round-end removal and, once the hold lifts, republishes the stopped
// listener's address over the held node. Windows-only because the hold is.
func TestServeErrorDrainsThePublicationWatchdog(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	addrDir := t.TempDir()
	c, err := securefiles.Open(addrDir)
	require.NoError(t, err, "Setup: could not create custodian")
	defer func() { _ = c.Close() }()

	addrPath := filepath.Join(addrDir, common.ListeningPortFileName)
	require.NoError(t, c.WriteFile(common.ListeningPortFileName, []byte("old-address")),
		"Setup: could not seed the address file")
	holder, err := os.Open(addrPath)
	require.NoError(t, err, "Setup: could not hold the address file open")

	registerer := func(context.Context, bool) *grpc.Server {
		// A server stopped before Serve: the serving goroutine exits on its
		// own, mid-round, after the publication watchdog has started.
		server := grpc.NewServer()
		server.Stop()
		return server
	}

	d := daemon.New(ctx, registerer, c)
	err = d.Serve(ctx, daemon.WithPublicationInterval(100*time.Millisecond))
	require.Error(t, err, "a server stopped before Serve must end the round with an error")
	require.ErrorContains(t, err, "gRPC serve error")

	// The round is over and the held node stands as it was: the round-end
	// removal answers ErrNodeHeld against the holder. The hold now lifts. A
	// watchdog that survived the round would republish the stopped listener's
	// address on its next tick; a drained one cannot.
	require.NoError(t, holder.Close(), "Setup: could not lift the hold")
	time.Sleep(500 * time.Millisecond)
	got, err := os.ReadFile(addrPath)
	require.NoError(t, err, "the held node must still stand")
	require.Equal(t, "old-address", string(got), "no publication may follow the round that owned it")
}
