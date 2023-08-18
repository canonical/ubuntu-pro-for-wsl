package microsoftstore_test

import (
	"testing"

	"github.com/canonical/ubuntu-pro-for-windows/windows-agent/internal/contracts/microsoftstore"
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
		"Error because there is no subscription":       {token: "not a real token", wantErr: true, wantDllErr: microsoftstore.ErrStoreAPI},
		"Error because the token has a null character": {token: "invalid \x00 token", wantErr: true},
	}

	for name, tc := range testCases {
		tc := tc
		t.Run(name, func(t *testing.T) {
			t.Parallel()

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
