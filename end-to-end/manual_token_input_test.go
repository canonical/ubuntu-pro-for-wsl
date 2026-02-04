package endtoend_test

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	wsl "github.com/ubuntu/gowsl"
)

func TestManualTokenInput(t *testing.T) {
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
			ctx := context.Background()

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

			// Either runs the ubuntupro app before...
			if tc.whenToken == beforeDistroRegistration {
				cleanup := startAgent(t, ctx, currentFuncName, tc.overrideTokenEnv)
				defer cleanup()
			}

			// Distro setup
			name := registerFromTestImage(t, ctx)
			d := wsl.NewDistro(ctx, name)

			defer logWslProServiceOnError(t, ctx, d)

			out, err := d.Command(ctx, "cloud-init status --wait --long").CombinedOutput()
			s := string(out)
			t.Logf("cloud-init output:\n%s", s)
			if err != nil {
				out2, err2 := d.Command(ctx, "cat /run/cloud/ds-identify.log").CombinedOutput()
				t.Logf("ds-identify:\n%s\n%s", err2, out2)
				out2, err2 = d.Command(ctx, "cat /var/log/cloud-init.log").CombinedOutput()
				t.Logf("cloud-init:\n%s\n%s", err2, out2)
			}
			require.Containsf(t, s, "status: done", "Setup: could not wake distro up: %v. %s", err, out)

			// ... or after registration, but never both.
			if tc.whenToken == afterDistroRegistration {
				cleanup := startAgent(t, ctx, currentFuncName, tc.overrideTokenEnv)
				defer cleanup()
				out, err = d.Command(ctx, "exit 0").CombinedOutput()
				require.NoErrorf(t, err, "Setup: could not wake distro up: %v. %s", err, out)
			}

			// By now the agent should have initialized the registry
			requireRegistryIsInitialized(t, []string{"UbuntuProToken", "LandscapeConfig"})
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
			}, maxTimeout, 20*time.Second, "distro should have been Pro attached")

			info := landscape.RequireReceivedInfo(t, os.Getenv(proTokenEnv), []wsl.Distro{d}, hostname)
			landscape.RequireUninstallCommand(t, ctx, d, info)
		})
	}
}
