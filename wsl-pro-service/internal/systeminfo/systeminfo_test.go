package systeminfo_test

import (
	"context"
	"errors"
	"testing"

	"github.com/canonical/ubuntu-pro-for-windows/wsl-pro-service/internal/systeminfo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGet(t *testing.T) {
	type functionMock int
	const (
		functionMockOK        functionMock = iota // A mock that mimicks the happy path
		functionMockError                         // A mock that returns an error
		functionMockBadReturn                     // A mock that returns a bad value with no error
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

		"Error when WSL_DISTRO_NAME is empty and wslpath fails":            {distroNameEnvDisabled: true, distroNameWslPath: functionMockError, wantErr: true},
		"Error when WSL_DISTRO_NAME is empty and wslpath returns bad text": {distroNameEnvDisabled: true, distroNameWslPath: functionMockBadReturn, wantErr: true},

		"Error when pro status command fails":           {proStatusCommand: functionMockError, wantErr: true},
		"Error when pro status output cannot be parsed": {proStatusCommand: functionMockBadReturn, wantErr: true},

		"Error when /etc/os-release cannot be read":       {osRelease: functionMockError, wantErr: true},
		"Error whem /etc/os-release returns bad contents": {osRelease: functionMockBadReturn, wantErr: true},
	}

	for name, tc := range testCases {
		tc := tc
		t.Run(name, func(t *testing.T) {
			if tc.distroNameEnvDisabled {
				t.Setenv(systeminfo.DistroNameEnv, "")
			} else {
				t.Setenv(systeminfo.DistroNameEnv, "TEST_DISTRO")
			}

			switch tc.distroNameWslPath {
			case functionMockOK:
				systeminfo.InjectWslRootPath(t, func() ([]byte, error) {
					return []byte(`\\wsl.localhost\TEST_DISTRO\`), nil
				})
			case functionMockError:
				systeminfo.InjectWslRootPath(t, func() ([]byte, error) {
					return nil, errors.New("test error")
				})
			case functionMockBadReturn:
				systeminfo.InjectWslRootPath(t, func() ([]byte, error) {
					return []byte(`Try and parse me`), nil
				})
			}

			switch tc.proStatusCommand {
			case functionMockOK:
				systeminfo.InjectProStatusCmdOutput(t, func(ctx context.Context) ([]byte, error) {
					return []byte(`{"attached": true, "anotherfield": "potato"}`), nil
				})
			case functionMockError:
				systeminfo.InjectProStatusCmdOutput(t, func(ctx context.Context) ([]byte, error) {
					return nil, errors.New("test error")
				})
			case functionMockBadReturn:
				systeminfo.InjectProStatusCmdOutput(t, func(ctx context.Context) ([]byte, error) {
					return []byte(`Parse me if you can`), nil
				})
			}

			switch tc.osRelease {
			case functionMockOK:
				systeminfo.InjectOsRelease(t, func() ([]byte, error) {
					return []byte(`PRETTY_NAME="Ubuntu 22.04.1 LTS"
NAME="Ubuntu"
VERSION_ID="22.04"
VERSION="22.04.1 LTS (Jammy Jellyfish)"
VERSION_CODENAME=jammy
ID=ubuntu
ID_LIKE=debian
HOME_URL="https://www.ubuntu.com/"
SUPPORT_URL="https://help.ubuntu.com/"
BUG_REPORT_URL="https://bugs.launchpad.net/ubuntu/"
PRIVACY_POLICY_URL="https://www.ubuntu.com/legal/terms-and-policies/privacy-policy"
UBUNTU_CODENAME=jammy"`), nil
				})
			case functionMockError:
				systeminfo.InjectOsRelease(t, func() ([]byte, error) {
					return nil, errors.New("test error")
				})
			case functionMockBadReturn:
				systeminfo.InjectOsRelease(t, func() ([]byte, error) {
					return []byte("Good luck parsing this"), nil
				})
			}

			sysinfo, err := systeminfo.Get()
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
