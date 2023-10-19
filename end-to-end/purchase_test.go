package endtoend_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/canonical/ubuntu-pro-for-windows/common/golden"
	"github.com/canonical/ubuntu-pro-for-windows/mocks/contractserver/contractsmockserver"
	"github.com/canonical/ubuntu-pro-for-windows/mocks/storeserver/storemockserver"
	"github.com/stretchr/testify/require"
	wsl "github.com/ubuntu/gowsl"
	"gopkg.in/yaml.v3"
)

const (
	contractsEndpointEnv     = "UP4W_CONTRACTS_BACKEND_MOCK_ENDPOINT"
	storeEndpointEnv         = "UP4W_MS_STORE_MOCK_ENDPOINT"
	allowPurchaseEnvOverride = "UP4W_ALLOW_STORE_PURCHASE=1"
)

func TestPurchase(t *testing.T) {
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
		tc := tc
		t.Run(name, func(t *testing.T) {
			testSetup(t)

			ctx := context.Background()
			contractsCtx, contractsCancel := context.WithCancel(ctx)
			defer contractsCancel()

			settings := contractsmockserver.DefaultSettings()

			token := os.Getenv(proTokenEnv)
			if tc.withToken != "" {
				token = tc.withToken
			}
			require.NotEmpty(t, token, "Provide a pro token either via UP4W_TEST_PRO_TOKEN environment variable or the test case struct withToken field")
			settings.Subscription.OnSuccess.Value = token

			cs := contractsmockserver.NewServer(settings)
			if !tc.csServerDown {
				err := cs.Serve(contractsCtx, "localhost:0")
				require.NoError(t, err, "Setup: Server should return no error")
			}
			contractsEndpointEnvOverride := fmt.Sprintf("%s=%s", contractsEndpointEnv, cs.Address())
			//nolint:errcheck // Nothing we can do about it
			defer cs.Stop()
			storeCtx, storeCancel := context.WithCancel(ctx)
			defer storeCancel()

			storeSettings := storemockserver.DefaultSettings()

			testData, err := os.ReadFile(filepath.Join(golden.TestFamilyPath(t), "storemock_config.yaml"))
			require.NoError(t, err, "Setup: Could not read test fixture input file")

			err = yaml.Unmarshal(testData, &storeSettings)
			require.NoError(t, err, "Setup: Unmarshalling test data should return no error")

			store := storemockserver.NewServer(storeSettings)
			if !tc.storeDown {
				err = store.Serve(storeCtx, "localhost:0")
				require.NoError(t, err, "Setup: Server should return no error")
			}
			storeEndpointEnvOverride := fmt.Sprintf("%s=%s", storeEndpointEnv, store.Address())
			//nolint:errcheck // Nothing we can do about it
			defer store.Stop()

			// Either runs the ubuntupro app before...
			if tc.whenToStartAgent == beforeDistroRegistration {
				cleanup := startAgent(t, ctx, currentFuncName, allowPurchaseEnvOverride, contractsEndpointEnvOverride, storeEndpointEnvOverride)
				defer cleanup()
			}

			// Distro setup
			name := registerFromTestImage(t, ctx)
			d := wsl.NewDistro(ctx, name)

			defer func() {
				if t.Failed() {
					logWslProServiceJournal(t, ctx, d)
				}
			}()

			out, err := d.Command(ctx, "exit 0").CombinedOutput()
			require.NoErrorf(t, err, "Setup: could not wake distro up: %v. %s", err, out)

			// ... or after registration, but never both.
			if tc.whenToStartAgent == afterDistroRegistration {
				cleanup := startAgent(t, ctx, currentFuncName, allowPurchaseEnvOverride, contractsEndpointEnvOverride, storeEndpointEnvOverride)
				defer cleanup()

				out, err = d.Command(ctx, "exit 0").CombinedOutput()
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
