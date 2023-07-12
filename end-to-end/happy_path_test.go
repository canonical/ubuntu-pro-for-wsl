package endtoend_test

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	wsl "github.com/ubuntu/gowsl"
	"golang.org/x/sys/windows/registry"
)

func TestProEnableDistro(t *testing.T) {
	ctx := context.Background()
	testSetup(t)

	token := os.Getenv(proTokenKey)
	require.NotEmptyf(t, token, "Setup: environment variable %q should contain a valid pro token, but is empty", proTokenKey)

	// Agent setup
	key, _, err := registry.CreateKey(registry.CURRENT_USER, registryPath, registry.WRITE)
	require.NoErrorf(t, err, "Setup: could not open UbuntuPro registry key")
	defer key.Close()

	err = key.SetStringValue("ProTokenOrg", token)
	require.NoError(t, err, "could not write token in registry")

	err = key.Close()
	require.NoError(t, err, "could not close registry key")

	// Distro setup
	name := registerFromGoldenImage(t, ctx)
	d := wsl.NewDistro(ctx, name)

	t.Log("Installing wsl-pro-service")
	out, err := d.Command(ctx, fmt.Sprintf("dpkg -i $(wslpath -ua '%s')", wslProServiceDebPath)).CombinedOutput()
	require.NoErrorf(t, err, "Setup: could not install wsl-pro-service: %v. %s", err, out)
	t.Log("Installed wsl-pro-service")

	defer logWslProServiceJournal(t, ctx, d)

	err = d.Terminate()
	require.NoError(t, err, "could not restart distro")

	// Start of test: start agent and mimic first boot
	startAgent(t, ctx)

	out, err = d.Command(ctx, "exit 0").CombinedOutput()
	require.NoErrorf(t, err, "Setup: could not wake distro up: %v. %s", err, out)

	// Give the agent some time to pro-attach
	time.Sleep(5 * time.Second)

	// Validate that the distro was attached
	out, err = d.Command(ctx, "pro status --format=json").Output()
	require.NoErrorf(t, err, "Setup: could not call pro status: %v. %s", err, out)

	var response struct {
		Attached bool
	}
	err = json.Unmarshal(out, &response)
	require.NoError(t, err, "could not parse pro status response: %s", out)
	require.True(t, response.Attached, "distro should have been Pro attached")
}

func logWslProServiceJournal(t *testing.T, ctx context.Context, d wsl.Distro) {
	out, err := d.Command(ctx, "journalctl --no-pager -u wsl-pro.service").CombinedOutput()
	if err != nil {
		t.Logf("could not access logs: %v\n%s\n", err, out)
	}
	t.Logf("wsl-pro-service logs:\n%s\n", out)
}
