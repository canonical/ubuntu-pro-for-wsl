package endtoend_test

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	wsl "github.com/ubuntu/gowsl"
	"golang.org/x/sys/windows/registry"
)

func TestOrganizationProvidedToken(t *testing.T) {
	type whenToken int
	const (
		never whenToken = iota
		beforeDistroRegistration
		afterDistroRegistration
	)

	// Let's be lazy and don't fall into the risk of changing the function name without updating the places where its name is used.
	currentFuncName := t.Name()

	testCases := map[string]struct {
		whenToken whenToken

		wantAttached bool
	}{
		"Success when the subscription is active before registration":   {whenToken: beforeDistroRegistration, wantAttached: true},
		"Success when the subscription is activated after registration": {whenToken: afterDistroRegistration, wantAttached: true},

		"Error when there is no active subscription": {whenToken: never, wantAttached: false},
	}

	for name, tc := range testCases {
		tc := tc
		t.Run(name, func(t *testing.T) {
			ctx := context.Background()

			testSetup(t)
			defer logWindowsAgentJournal(t, true)

			if tc.whenToken == beforeDistroRegistration {
				activateOrgSubscription(t)
				cleanup := startAgent(t, ctx, currentFuncName)
				defer cleanup()
			}

			// Distro setup
			name := registerFromTestImage(t, ctx)
			d := wsl.NewDistro(ctx, name)

			defer logWslProServiceOnError(t, ctx, d)

			out, err := d.Command(ctx, "exit 0").CombinedOutput()
			require.NoErrorf(t, err, "Setup: could not wake distro up: %v. %s", err, out)

			if tc.whenToken == afterDistroRegistration {
				err := d.Terminate()
				require.NoError(t, err, "could not restart distro")

				activateOrgSubscription(t)
				cleanup := startAgent(t, ctx, currentFuncName)
				defer cleanup()

				out, err := d.Command(ctx, "exit 0").CombinedOutput()
				require.NoErrorf(t, err, "Setup: could not wake distro up: %v. %s", err, out)
			}

			const maxTimeout = 30 * time.Second

			if !tc.wantAttached {
				time.Sleep(maxTimeout)
				proCtx, cancel := context.WithTimeout(ctx, maxTimeout)
				defer cancel()
				attached, err := distroIsProAttached(t, proCtx, d)
				require.NoError(t, err, "could not determine if distro is attached")
				require.False(t, attached, "distro should not have been Pro attached")
				return
			}

			require.Eventually(t, func() bool {
				attached, err := distroIsProAttached(t, ctx, d)
				if err != nil {
					t.Logf("could not determine if distro is attached: %v", err)
				}
				return attached
			}, maxTimeout, time.Second, "distro should have been Pro attached")
		})
	}
}

func activateOrgSubscription(t *testing.T) {
	t.Helper()

	token := os.Getenv(proTokenEnv)
	require.NotEmptyf(t, token, "Setup: environment variable %q should contain a valid pro token, but is empty", proTokenEnv)

	key, _, err := registry.CreateKey(registry.CURRENT_USER, registryPath, registry.WRITE)
	require.NoErrorf(t, err, "Setup: could not open UbuntuPro registry key")
	defer key.Close()

	err = key.SetStringValue("UbuntuProToken", token)
	require.NoError(t, err, "could not write token in registry")
}
