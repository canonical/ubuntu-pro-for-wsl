package endtoend_test

import (
	"context"
	"os/exec"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	wsl "github.com/ubuntu/gowsl"
)

func TestManualTokenInputSkipLandscape(t *testing.T) {
	// Let's be lazy and don't fall into the risk of changing the function name without updating the places where its name is used.
	currentFuncName := t.Name()

	ctx := t.Context()

	testSetup(t)
	defer logWindowsAgentOnError(t)

	// Distro setup
	name := registerFromTestImage(t, ctx)
	d := wsl.NewDistro(ctx, name)
	// t.Context() is still valid when deferred functions are executed.
	defer logWslProServiceOnError(t, ctx, d)
	defer logProClientOnError(t, d.Name())

	cmdCtx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()
	// Make sure the instance is fully provisioned.
	// #nosec G204 // The distro name is controlled by our tests.
	cmd := exec.CommandContext(cmdCtx, "wt.exe", "wsl.exe", "-d", name)
	require.NoError(t, cmd.Start(), "Setup: could not start instance %s", name)
	time.Sleep(1 * time.Second)
	//nolint:errcheck // There is nothing we can do if this fails.
	defer cmd.Process.Kill()
	// #nosec G204 // The distro name is controlled by our tests.
	out, err := exec.CommandContext(cmdCtx, "wsl.exe", "-d", name, "cloud-init", "status", "--wait").CombinedOutput()
	require.NoErrorf(t, err, "Setup: could not wake distro up: %v. %s", err, out)

	cleanup := startAgent(t, ctx, currentFuncName)
	defer cleanup()

	// By now the agent should have initialized the registry with empty values.
	requireRegistryIsInitialized(t, []string{"UbuntuProToken", "LandscapeConfig"})
	const maxTimeout = 2 * time.Minute

	require.Eventually(t, func() bool {
		tried, err := triedProAttach(t, d.Name())
		if err != nil {
			t.Logf("could not determine if distro tried to attach to Pro: %v", err)
			return false
		}
		t.Logf("checking if distro instance tried to attach to Pro: %v", tried)
		return tried
	}, maxTimeout, 10*time.Second, "The distro instance did not try to attach to Pro")
}
