package endtoend_test

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	landscapeapi "github.com/canonical/landscape-hostagent-api"
	"github.com/canonical/ubuntu-pro-for-wsl/common/testutils"
	"github.com/stretchr/testify/require"
	wsl "github.com/ubuntu/gowsl"
)

func TestCloudInitIntegration(t *testing.T) {
	deinit, err := initializeCOM()
	if err != nil {
		deinit()
		t.Fatalf("could not initialize COM: %v", err)
	}
	defer deinit()

	currentFuncName := t.Name()

	ctx := t.Context()

	testSetup(t)
	defer logWindowsAgentOnError(t)

	landscape := NewLandscape(t, ctx)
	writeUbuntuProRegistry(t, "LandscapeConfig", landscape.ClientConfig)

	serverDone := make(chan struct{})
	go func() {
		defer close(serverDone)
		landscape.Serve()
	}()
	t.Cleanup(func() {
		landscape.Stop()
		<-serverDone
	})
	defer landscape.LogOnError(t)

	hostname, err := os.Hostname()
	require.NoError(t, err, "Setup: could not test machine's hostname")

	proToken := os.Getenv(proTokenEnv)
	require.NotEmptyf(t, proToken, "Setup: environment variable %q should contain a valid pro token, but is empty", proTokenEnv)
	writeUbuntuProRegistry(t, "UbuntuProToken", proToken)

	cleanup := startAgent(t, ctx, currentFuncName)
	defer cleanup()

	info := landscape.RequireReceivedInfo(t, proToken, nil, hostname)

	out, err := os.ReadFile(filepath.Join(testutils.TestFixturePath(t), "user-data.yaml"))
	require.NoError(t, err, "Setup: could not read cloud-init file")
	cloudInitUserData := string(out)

	name := "Ubuntu-Preview"
	err = landscape.service.SendCommand(ctx, info.UID, &landscapeapi.Command{
		Cmd: &landscapeapi.Command_Install_{
			Install: &landscapeapi.Command_Install{
				Id:        name,
				Cloudinit: &cloudInitUserData,
			},
		},
		RequestId: "Server123",
	})
	require.NoError(t, err, "Setup: could not send install command")

	distro := wsl.NewDistro(ctx, name)

	//nolint:errcheck // Nothing we can do about it
	defer distro.Unregister()

	require.Eventually(t, func() bool {
		ok, err := distro.IsRegistered()
		if err != nil {
			t.Logf("could not determine if distro is registered: %v", err)
			return false
		}
		if !ok {
			return false
		}
		state, err := distro.State()
		if err != nil {
			t.Logf("Could not determine if distro is registered: %v", err)
			return false
		}
		return state == wsl.Stopped
	}, 10*time.Minute, 10*time.Second, "Distro should have been registered")
	t.Log(runCommand(t, ctx, time.Minute, distro, "cloud-init status --wait"))

	defer logWslProServiceOnError(t, ctx, distro)
	defer logProClientOnError(t, distro.Name())

	require.Eventually(t, func() bool {
		attached, err1 := distroIsProAttached(t, ctx, distro)
		tried, err2 := triedProAttach(t, distro.Name())
		if err1 != nil && err2 != nil {
			t.Logf("Could not determine if distro tried to attach to Pro: %v", errors.Join(err1, err2))
			return false
		}
		return attached || tried
	}, 10*time.Second, time.Second, "distro should have tried to attach to Pro")

	// Finally, wake the distro instance so wsl-pro-service can talk to the agent.
	cmd := exec.CommandContext(ctx, "wsl.exe", "-d", name)
	require.NoError(t, cmd.Start(), "Could not launch the distro for final assertions")
	//nolint:errcheck // There is nothing we can do if this fails.
	defer cmd.Process.Kill()

	uid, err := distro.Command(ctx, "id -u testuser").CombinedOutput()
	require.NoError(t, err, "cloud-init should have configured the default user, uid is %s", uid)

	landscape.RequireReceivedInfo(t, proToken, []wsl.Distro{distro}, hostname)
	landscape.RequireUninstallCommand(t, ctx, distro, info)
}

//nolint:revive // t always goes before ctx
func runCommand(t *testing.T, ctx context.Context, timeout time.Duration, distro wsl.Distro, command string) string {
	t.Helper()

	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	out, err := distro.Command(ctx, command).CombinedOutput()
	if err == nil {
		return strings.TrimSpace(string(out))
	}

	// We check the context to see if it was a timeout.
	// This makes the error message easier to understand.
	select {
	case <-ctx.Done():
		require.Fail(t, "Timed out waiting for cloud-init to finish")
	default:
	}

	return fmt.Sprintf("Stdout: %s. Stderr: %s", out, err)
}
