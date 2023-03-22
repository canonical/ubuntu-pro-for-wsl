package systeminfo_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/canonical/ubuntu-pro-for-windows/wsl-pro-service/internal/systeminfo"
	"github.com/canonical/ubuntu-pro-for-windows/wsl-pro-service/internal/testutils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGet(t *testing.T) {
	type functionMock int
	const (
		mockOK        functionMock = iota // A mock that mimicks the happy path
		mockError                         // A mock that returns an error
		mockBadReturn                     // A mock that returns a bad value with no error
	)

	testCases := map[string]struct {
		// This causes Get to look at the Windows' path for /
		distroNameEnvDisabled bool

		// The path of "/" according to the Windows host
		distroNameWslPath functionMock

		proStatusCommand functionMock
		osRelease        functionMock

		wantErr bool
	}{
		"Success reading from WSL_DISTRO_NAME": {},
		"Success using wslpath":                {distroNameEnvDisabled: true},

		"Error when WSL_DISTRO_NAME is empty and wslpath fails":            {distroNameEnvDisabled: true, distroNameWslPath: mockError, wantErr: true},
		"Error when WSL_DISTRO_NAME is empty and wslpath returns bad text": {distroNameEnvDisabled: true, distroNameWslPath: mockBadReturn, wantErr: true},

		"Error when pro status command fails":           {proStatusCommand: mockError, wantErr: true},
		"Error when pro status output cannot be parsed": {proStatusCommand: mockBadReturn, wantErr: true},

		"Error when /etc/os-release cannot be read":       {osRelease: mockError, wantErr: true},
		"Error whem /etc/os-release returns bad contents": {osRelease: mockBadReturn, wantErr: true},
	}

	for name, tc := range testCases {
		tc := tc
		t.Run(name, func(t *testing.T) {
			if tc.distroNameEnvDisabled {
				t.Setenv(systeminfo.DistroNameEnv, "")
			} else {
				t.Setenv(systeminfo.DistroNameEnv, "TEST_DISTRO")
			}

			rootDir := testutils.MockFilesystem(t)

			switch tc.distroNameWslPath {
			case mockOK:
				systeminfo.InjectWslRootPath(t, func() ([]byte, error) {
					return []byte(`\\wsl.localhost\TEST_DISTRO\`), nil
				})
			case mockError:
				systeminfo.InjectWslRootPath(t, func() ([]byte, error) {
					return nil, errors.New("test error")
				})
			case mockBadReturn:
				systeminfo.InjectWslRootPath(t, func() ([]byte, error) {
					return []byte(`Try and parse me`), nil
				})
			}

			switch tc.proStatusCommand {
			case mockOK:
				systeminfo.InjectProStatusCmdOutput(t, func(ctx context.Context) ([]byte, error) {
					return []byte(`{"attached": true, "anotherfield": "potato"}`), nil
				})
			case mockError:
				systeminfo.InjectProStatusCmdOutput(t, func(ctx context.Context) ([]byte, error) {
					return nil, errors.New("test error")
				})
			case mockBadReturn:
				systeminfo.InjectProStatusCmdOutput(t, func(ctx context.Context) ([]byte, error) {
					return []byte(`Parse me if you can`), nil
				})
			}

			switch tc.osRelease {
			case mockOK:
			case mockError:
				os.Remove(filepath.Join(rootDir, "/etc/os-release"))
			case mockBadReturn:
				os.WriteFile(filepath.Join(rootDir, "/etc/os-release"), []byte("This file has the wrong syntax"), 0600)
			}

			sysinfo, err := systeminfo.Get(rootDir)
			if tc.wantErr {
				require.Error(t, err, "Expected Get() to return an error")
				return
			}
			require.NoError(t, err, "Expected Get() to return no errors")

			assert.Equal(t, "TEST_DISTRO", sysinfo.WslName, "WslName does not match expected value")
			assert.Equal(t, "ubuntu", sysinfo.Id, "Id does not match expected value")
			assert.Equal(t, "22.04", sysinfo.VersionId, "VersionId does not match expected value")
			assert.Equal(t, "Ubuntu 22.04.1 LTS", sysinfo.PrettyName, "PrettyName does not match expected value")
			assert.Equal(t, true, sysinfo.ProAttached, "ProAttached does not match expected value")
		})
	}
}
