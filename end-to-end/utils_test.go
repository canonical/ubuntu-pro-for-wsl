package endtoend_test

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io/fs"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/canonical/ubuntu-pro-for-windows/common/wsltestutils"
	"github.com/stretchr/testify/require"
	"github.com/ubuntu/gowsl"
)

func testSetup(t *testing.T) {
	t.Helper()
	ctx := context.Background()

	err := gowsl.Shutdown(ctx)
	require.NoError(t, err, "Setup: could not shut WSL down")

	err = assertCleanRegistry()
	require.NoError(t, err, "Setup: registry is polluted, potentially by a previous test")

	err = assertCleanLocalAppData()
	require.NoError(t, err, "Setup: local app data is polluted, potentially by a previous test")

	err = stopAgent(ctx)
	require.NoError(t, err, "Setup: could not stop the agent")

	t.Cleanup(func() {
		err := errors.Join(
			cleanupRegistry(),
			cleanupLocalAppData(),
			stopAgent(ctx),
		)
		// Cannot assert: the test is finished already
		if err != nil {
			log.Printf("Cleanup: %v", err)
		}
	})
}

//nolint:revive // testing.T must precede the context
func registerFromTestImage(t *testing.T, ctx context.Context) string {
	t.Helper()

	distroName := wsltestutils.RandomDistroName(t)
	t.Logf("Registering distro %q", distroName)
	defer t.Logf("Registered distro %q", distroName)

	_ = wsltestutils.PowershellImportDistro(t, ctx, distroName, testImagePath)
	return distroName
}

// startAgent starts the GUI (without interacting with it) and waits for the Agent to start.
// It stops the agent upon cleanup. If the cleanup fails, the testing will be stopped.
//
//nolint:revive // testing.T must precede the contex
func startAgent(t *testing.T, ctx context.Context) (cleanup func()) {
	t.Helper()

	t.Log("Starting agent")

	out, err := powershellf(ctx, "(Get-AppxPackage CanonicalGroupLimited.UbuntuProForWindows).InstallLocation").CombinedOutput()
	require.NoError(t, err, "could not locate ubuntupro.exe: %v. %s", err, out)

	ubuntupro := filepath.Join(strings.TrimSpace(string(out)), "gui", "ubuntupro.exe")
	//nolint:gosec // The executable is located at the Appx directory
	cmd := exec.CommandContext(ctx, ubuntupro)

	var buff bytes.Buffer
	cmd.Stdout = &buff
	cmd.Stderr = &buff

	err = cmd.Start()
	require.NoError(t, err, "Setup: could not start agent")

	cleanup = func() {
		t.Log("Cleanup: stopping agent process")

		if err := stopAgent(ctx); err != nil {
			// We have to abort because the tests become coupled via the agent
			log.Fatalf("Could not kill ubuntu-pro-agent process: %v: %s", err, out)
		}

		//nolint:errcheck // Nothing we can do about it
		cmd.Process.Kill()

		//nolint:errcheck // We know that the previous "Kill" stopped it
		cmd.Wait()
		t.Logf("Agent stopped. Stdout+stderr: %s", buff.String())
	}

	defer func() {
		if t.Failed() {
			cleanup()
		}
	}()

	require.Eventually(t, func() bool {
		localAppData := os.Getenv("LocalAppData")
		if localAppData == "" {
			t.Logf("Agent setup: $env:LocalAppData should not be empty")
			return false
		}

		_, err := os.Stat(filepath.Join(localAppData, "Ubuntu Pro", "addr"))
		if errors.Is(err, fs.ErrNotExist) {
			return false
		}
		if err != nil {
			t.Logf("Agent setup: could not read addr file: %v", err)
			return false
		}
		return true
	}, 10*time.Second, 100*time.Millisecond, "Agent never started serving")

	t.Log("Started agent")
	return cleanup
}

// stopAgent kills the process for the Windows Agent.
func stopAgent(ctx context.Context) error {
	const process = "ubuntu-pro-agent"

	out, err := powershellf(ctx, "Stop-Process -ProcessName %q", process).CombinedOutput()
	if err == nil {
		return nil
	}

	if strings.Contains(string(out), "NoProcessFoundForGivenName") {
		return nil
	}

	return fmt.Errorf("could not stop process %q: %v. %s", process, err, out)
}
