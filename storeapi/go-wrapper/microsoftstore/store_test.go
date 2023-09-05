package microsoftstore_test

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/canonical/ubuntu-pro-for-windows/common"
	"github.com/canonical/ubuntu-pro-for-windows/storeapi/go-wrapper/microsoftstore"
	"github.com/stretchr/testify/require"
)

func TestMain(m *testing.M) {
	ctx := context.Background()

	if runtime.GOOS == "windows" {
		if err := buildStoreAPI(ctx); err != nil {
			fmt.Fprintf(os.Stderr, "Setup: %v", err)
			os.Exit(1)
		}
	}

	exit := m.Run()
	defer os.Exit(exit)
}

func TestGenerateUserJWT(t *testing.T) {
	t.Parallel()

	if runtime.GOOS != "windows" {
		t.Skip("This test is only supported on Windows")
	}

	testCases := map[string]struct {
		token string

		wantErr    bool
		wantDllErr microsoftstore.StoreAPIError
	}{
		"Error because there is no subscription":       {token: "not a real token", wantErr: true, wantDllErr: microsoftstore.ErrEmptyJwt},
		"Error because the token has a null character": {token: "invalid \x00 token", wantErr: true},
	}

	for name, tc := range testCases {
		tc := tc
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			if tc.wantDllErr != microsoftstore.ErrSuccess && isGithubRunner() {
				// Github runners run on session 0, so the error is a lack of user ID
				tc.wantDllErr = microsoftstore.ErrNoLocalUser
			}

			jwt, err := microsoftstore.GenerateUserJWT(tc.token)
			if tc.wantErr {
				require.Error(t, err, "SubscriptionToken should not succeed")
				if tc.wantDllErr != 0 {
					require.ErrorIs(t, err, tc.wantDllErr, "SubscriptionToken returned an unexpected error type")
				}
				return
			}

			require.NoError(t, err, "SubscriptionToken should succeed")
			require.NotEmpty(t, jwt, "User JWToken should not be empty")
		})
	}
}

func TestGetSubscriptionExpirationDate(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("This test is only supported on Windows")
	}

	wantErr := microsoftstore.ErrNoProductsFound

	_, gotErr := microsoftstore.GetSubscriptionExpirationDate()
	require.ErrorIs(t, gotErr, wantErr, "GetSubscriptionExpirationDate should have returned code %d", wantErr)
}

func buildStoreAPI(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()

	root, err := common.FindWorkspaceRoot()
	if err != nil {
		return err
	}

	//nolint:gosec // Only used in tests.
	cmd := exec.CommandContext(ctx, "msbuild",
		filepath.Join(root, `/msix/storeapi/storeapi.vcxproj`),
		`-target:Build`,
		`-property:Configuration=Debug`,
		`-property:Platform=x64`,
		`-nologo`,
		`-verbosity:normal`,
	)

	log.Printf("Building store api DLL")

	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("could not build store api DLL: %v. Log:\n%s", err, out)
	}

	log.Printf("Built store api DLL")

	return nil
}

func isGithubRunner() bool {
	return os.Getenv("GITHUB_WORKFLOW") != ""
}
