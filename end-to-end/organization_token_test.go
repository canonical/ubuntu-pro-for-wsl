package endtoend_test

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	wsl "github.com/ubuntu/gowsl"
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
		t.Run(name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			testSetup(t)
			defer logWindowsAgentOnError(t)

			landscape := NewLandscape(t, ctx)
			writeUbuntuProRegistry(t, "LandscapeConfig", landscape.ClientConfig)

			go landscape.Serve()
			defer landscape.LogOnError(t)
			defer landscape.Stop()

			hostname, err := os.Hostname()
			require.NoError(t, err, "Setup: could not test machine's hostname")

			proToken := os.Getenv(proTokenEnv)
			require.NotEmptyf(t, proToken, "Setup: environment variable %q should contain a valid pro token, but is empty", proTokenEnv)
			writeUbuntuProRegistry(t, "UbuntuProToken", proToken)

			if tc.whenToken == beforeDistroRegistration {
				cleanup := startAgent(t, ctx, currentFuncName)
				defer cleanup()
			}

			// Distro setup
			name := registerFromTestImage(t, ctx)
			d := wsl.NewDistro(ctx, name)

			defer logWslProServiceOnError(t, ctx, d)

			out, err := d.Command(ctx, "cloud-init status --wait").CombinedOutput()
			require.NoErrorf(t, err, "Setup: could not wake distro up: %v. %s", err, out)

			if tc.whenToken == afterDistroRegistration {
				err := d.Terminate()
				require.NoError(t, err, "could not restart distro")

				cleanup := startAgent(t, ctx, currentFuncName)
				defer cleanup()

				out, err := d.Command(ctx, "exit 0").CombinedOutput()
				require.NoErrorf(t, err, "Setup: could not wake distro up: %v. %s", err, out)
			}

			const maxTimeout = time.Minute

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

			info := landscape.RequireReceivedInfo(t, proToken, []wsl.Distro{d}, hostname)
			landscape.RequireUninstallCommand(t, ctx, d, info)
		})
	}
}
