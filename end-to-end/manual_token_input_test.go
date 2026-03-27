package endtoend_test

import (
	"context"
	"fmt"
	"os/exec"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	wsl "github.com/ubuntu/gowsl"
)

func TestManualTokenInputSkipLandscape(t *testing.T) {
	type whenToken int
	const (
		never whenToken = iota
		beforeDistroRegistration
		afterDistroRegistration
	)

	// Let's be lazy and don't fall into the risk of changing the function name without updating the places where its name is used.
	currentFuncName := t.Name()

	testCases := map[string]struct {
		whenToken        whenToken
		overrideTokenEnv string

		wantAttached bool
	}{
		"Success when applying pro token before registration": {whenToken: beforeDistroRegistration, wantAttached: true},
		"Success when applying pro token after registration":  {whenToken: afterDistroRegistration, wantAttached: true},

		"Error with invalid token": {whenToken: afterDistroRegistration, overrideTokenEnv: fmt.Sprintf("%s=%s", proTokenEnv, "CJd8MMN8wXSWsv7wJT8c8dDK")},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			ctx := t.Context()

			testSetup(t)
			defer logWindowsAgentOnError(t)

			// Either runs the ubuntupro app before...
			if tc.whenToken == beforeDistroRegistration {
				cleanup := startAgent(t, ctx, currentFuncName, tc.overrideTokenEnv)
				defer cleanup()
			}

			// Distro setup
			name := registerFromTestImage(t, ctx)
			d := wsl.NewDistro(ctx, name)

			defer logWslProServiceOnError(t, ctx, d)

			// Make sure the instance is fully provisioned.
			// #nosec G204 // The distro name is controlled by our tests.
			cmd := exec.CommandContext(ctx, "wt.exe", "wsl.exe", "-d", name)
			require.NoError(t, cmd.Start(), "Setup: could not start instance %s", name)
			//nolint:errcheck // There is nothing we can do if this fails.
			defer cmd.Process.Kill()
			out, err := d.Command(ctx, "cloud-init status --wait").CombinedOutput()
			require.NoErrorf(t, err, "Setup: could not wake distro up: %v. %s", err, out)

			// ... or after registration, but never both.
			if tc.whenToken == afterDistroRegistration {
				cleanup := startAgent(t, ctx, currentFuncName, tc.overrideTokenEnv)
				defer cleanup()
			}

			// By now the agent should have initialized the registry with empty values.
			requireRegistryIsInitialized(t, []string{"UbuntuProToken", "LandscapeConfig"})
			const maxTimeout = 2 * time.Minute

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
			}, maxTimeout, 10*time.Second, "distro should have been Pro attached")
		})
	}
}
