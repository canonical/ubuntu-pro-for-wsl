package endtoend_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	landscapeapi "github.com/canonical/landscape-hostagent-api"
	"github.com/canonical/ubuntu-pro-for-wsl/common/testutils"
	"github.com/stretchr/testify/require"
	wsl "github.com/ubuntu/gowsl"
)

func TestCloudInitIntegration(t *testing.T) {
	// TODO: Remove this line when cloud-init support for UP4W is released.
	// Follow this PR for more information: https://github.com/canonical/cloud-init/pull/5116
	t.Skip("This test depends on cloud-init support for UP4W being released.")
	currentFuncName := t.Name()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

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

	err = landscape.service.SendCommand(ctx, info.UID, &landscapeapi.Command{
		Cmd: &landscapeapi.Command_Install_{
			Install: &landscapeapi.Command_Install{
				Id:        referenceDistro,
				Cloudinit: &cloudInitUserData,
			},
		},
	})
	require.NoError(t, err, "Setup: could not send install command")

	distro := wsl.NewDistro(ctx, referenceDistro)

	//nolint:errcheck // Nothing we can do about it
	defer distro.Unregister()

	require.Eventually(t, func() bool {
		ok, err := distro.IsRegistered()
		if err != nil {
			t.Logf("could not determine if distro is registered: %v", err)
			return false
		}
		return ok
	}, time.Minute, time.Second, "Distro should have been registered")

	defer logWslProServiceOnError(t, ctx, distro)

	runCommand(t, ctx, time.Minute, distro, "cloud-init status --wait")

	require.Eventually(t, func() bool {
		attached, err := distroIsProAttached(t, ctx, distro)
		if err != nil {
			t.Logf("could not determine if distro is attached: %v", err)
			return false
		}
		return attached
	}, 10*time.Second, time.Second, "distro should have been Pro attached")

	userName := runCommand(t, ctx, 10*time.Second, distro, "whoami")
	require.Equal(t, "testuser", userName, "cloud-init should have configured the default user")

	landscape.RequireReceivedInfo(t, proToken, []wsl.Distro{distro}, hostname)
	landscape.RequireUninstallCommand(t, ctx, distro, info)
}

//nolint:revive // t always goes before ctx
func runCommand(t *testing.T, ctx context.Context, timeout time.Duration, distro wsl.Distro, comand string) string {
	t.Helper()

	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	out, err := distro.Command(ctx, comand).CombinedOutput()
	if err == nil {
		return string(out)
	}

	// We check the context to see if it was a timeout.
	// This makes the error message easier to understand.

	select {
	case <-ctx.Done():
		require.Fail(t, "Timed out waiting for cloud-init to finish")
	default:
	}

	require.NoError(t, err, "could not determine if cloud-init is done: %s. Output: %s", out, out)
	return ""
}
