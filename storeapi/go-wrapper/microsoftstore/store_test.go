package microsoftstore_test

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"syscall"
	"testing"
	"time"

	"github.com/canonical/ubuntu-pro-for-wsl/storeapi/go-wrapper/microsoftstore"
	"github.com/stretchr/testify/require"
)

func TestMain(m *testing.M) {
	ctx := context.Background()

	h := slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	})

	slog.SetDefault(slog.New(h))

	if runtime.GOOS == "windows" {
		if err := buildStoreAPI(ctx); err != nil {
			slog.Error(fmt.Sprintf("Setup: %v", err))
			os.Exit(1)
		}
	}

	m.Run()
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

func TestErrorVerification(t *testing.T) {
	t.Parallel()
	testcases := map[string]struct {
		hresult int64
		err     error

		wantErr bool
	}{
		"Success": {},
		// If HRESULT is not in the Store API error range and err is not a syscall.Errno then we don't have an error.
		"With an unknown value (not an error)": {hresult: 1, wantErr: false},

		"Upper bound of the Store API enum range": {hresult: -1, wantErr: true},
		"Lower bound of the Store API enum range": {hresult: int64(microsoftstore.ErrNotSubscribed), wantErr: true},
		"With a system error (errno)":             {hresult: 32 /*garbage*/, err: syscall.Errno(2) /*E_FILE_NOT_FOUND*/, wantErr: true},
		"With a generic (unreachable) error":      {hresult: 1, err: errors.New("test error"), wantErr: true},
		// This would mean an API call returning a non-error hresult plus GetLastError() returning ERROR_SUCCESS
		// This shouldn't happen in the current store API implementation anyway.
		"With weird successful error": {hresult: 1, err: syscall.Errno(0) /*ERROR_SUCCESS*/},
	}
	for name, tc := range testcases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			res, err := microsoftstore.CheckError(tc.hresult, tc.err)
			if tc.wantErr {
				require.Error(t, err, "CheckError should have returned an error for value: %v, returned value was: %v", tc.hresult, res)
				return
			}
			require.NoError(t, err, "CheckError should have not returned an error for value: %v, returned value was: %v", tc.hresult, res)
		})
	}
}

func buildStoreAPI(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()

	root, err := microsoftstore.FindWorkspaceRoot()
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

	slog.Info("Building store api DLL")

	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("could not build store api DLL: %v. Log:\n%s", err, out)
	}

	slog.Info("Built store api DLL")

	return nil
}

func isGithubRunner() bool {
	return os.Getenv("GITHUB_WORKFLOW") != ""
}
