package endtoend_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/canonical/ubuntu-pro-for-wsl/common/testutils"
	"github.com/canonical/ubuntu-pro-for-wsl/mocks/contractserver/contractsmockserver"
	"github.com/canonical/ubuntu-pro-for-wsl/mocks/storeserver/storemockserver"
	"github.com/stretchr/testify/require"
	wsl "github.com/ubuntu/gowsl"
	"go.yaml.in/yaml/v3"
)

const (
	contractsEndpointEnv     = "UP4W_CONTRACTS_BACKEND_MOCK_ENDPOINT"
	storeEndpointEnv         = "UP4W_MS_STORE_MOCK_ENDPOINT"
	allowPurchaseEnvOverride = "UP4W_ALLOW_STORE_PURCHASE=1"
)

func TestPurchase(t *testing.T) {
	// TODO: Remove this line when cloud-init support for UP4W is released.
	// Follow this PR for more information: https://github.com/canonical/cloud-init/pull/5116
	t.Skip("This test depends on cloud-init support for UP4W being released.")

	type whenToken int
	const (
		never whenToken = iota
		beforeDistroRegistration
		afterDistroRegistration
	)

	// Let's be lazy and don't fall into the risk of changing the function name without updating the places where its name is used.
	currentFuncName := t.Name()

	testCases := map[string]struct {
		whenToStartAgent whenToken
		withToken        string
		csServerDown     bool
		storeDown        bool

		wantAttached bool
	}{
		"Success when applying pro token before registration": {whenToStartAgent: beforeDistroRegistration, wantAttached: true},
		"Success when applying pro token after registration":  {whenToStartAgent: afterDistroRegistration, wantAttached: true},

		"Error due MS Store API failure":          {whenToStartAgent: beforeDistroRegistration, storeDown: true},
		"Error due Contracts Server Backend down": {whenToStartAgent: afterDistroRegistration, csServerDown: true},
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

			settings := contractsmockserver.DefaultSettings()

			token := os.Getenv(proTokenEnv)
			if tc.withToken != "" {
				token = tc.withToken
			}
			require.NotEmpty(t, token, "Setup: provide a Pro token either via UP4W_TEST_PRO_TOKEN environment variable or the test case struct withToken field")
			settings.Subscription.OnSuccess.Value = token

			cs := contractsmockserver.NewServer(settings)
			//nolint:errcheck // Nothing we can do about it
			defer cs.Stop()

			contractsCtx, contractsCancel := context.WithCancel(ctx)
			defer contractsCancel()

			if !tc.csServerDown {
				err := cs.Serve(contractsCtx, "localhost:0")
				require.NoError(t, err, "Setup: Server should return no error")
			}

			contractsEndpointEnvOverride := fmt.Sprintf("%s=%s", contractsEndpointEnv, cs.Address())

			testData, err := os.ReadFile(filepath.Join(testutils.TestFamilyPath(t), "storemock_config.yaml"))
			require.NoError(t, err, "Setup: Could not read test fixture input file")

			storeSettings := storemockserver.DefaultSettings()
			err = yaml.Unmarshal(testData, &storeSettings)
			require.NoError(t, err, "Setup: Unmarshalling test data should return no error")

			store := storemockserver.NewServer(storeSettings)
			//nolint:errcheck // Nothing we can do about it
			defer store.Stop()

			storeCtx, storeCancel := context.WithCancel(ctx)
			defer storeCancel()

			if !tc.storeDown {
				err = store.Serve(storeCtx, "localhost:0")
				require.NoError(t, err, "Setup: Server should return no error")
			}

			storeEndpointEnvOverride := fmt.Sprintf("%s=%s", storeEndpointEnv, store.Address())

			// Either runs the ubuntupro app before...
			if tc.whenToStartAgent == beforeDistroRegistration {
				cleanup := startAgent(t, ctx, currentFuncName, allowPurchaseEnvOverride, contractsEndpointEnvOverride, storeEndpointEnvOverride)
				defer cleanup()
			}

			// Distro setup
			name := registerFromTestImage(t, ctx)
			d := wsl.NewDistro(ctx, name)

			defer logWslProServiceOnError(t, ctx, d)

			out, err := d.Command(ctx, "cloud-init status --wait").CombinedOutput()
			require.NoErrorf(t, err, "Setup: could not wake distro up: %v. %s", err, out)

			// ... or after registration, but never both.
			if tc.whenToStartAgent == afterDistroRegistration {
				cleanup := startAgent(t, ctx, currentFuncName, allowPurchaseEnvOverride, contractsEndpointEnvOverride, storeEndpointEnvOverride)
				defer cleanup()

				out, err = d.Command(ctx, "exit 0").CombinedOutput()
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

			landscape.RequireReceivedInfo(t, token, []wsl.Distro{d}, hostname)
			// Skipping because we know it to be broken
			// See https://warthogs.atlassian.net/browse/UDENG-1810
			//
			// landscape.RequireUninstallCommand(t, ctx, d, info)
		})
	}
}
