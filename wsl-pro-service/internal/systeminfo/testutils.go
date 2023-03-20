package systeminfo

import (
	"context"
	"os"
	"testing"

	agentapi "github.com/canonical/ubuntu-pro-for-windows/agentapi/go"
	"github.com/canonical/ubuntu-pro-for-windows/wsl-pro-service/internal/testutils"
	"github.com/stretchr/testify/require"
)

// InjectWslRootPath changes the definition of private function wslRootPath.
// It is restored during test cleanup.
var InjectWslRootPath = testutils.DefineInjector(&wslRootPath)

// InjectOsRelease changes the definition of private function osRelease.
// It is restored during test cleanup.
var InjectOsRelease = testutils.DefineInjector(&osRelease)

// InjectProStatusCmdOutput changes the definition of private function proStatusCmdOutput.
// It is restored during test cleanup.
var InjectProStatusCmdOutput = testutils.DefineInjector(&proStatusCmdOutput)

// InjectMock injects the necessary dependencies to systeminfo
// in order to make its functions platform independent.
//
// It returns the expected DistroInfo.
func InjectMock(t *testing.T) *agentapi.DistroInfo {
	t.Helper()

	err := os.Setenv(DistroNameEnv, "")
	require.NoError(t, err, "Setup: could not override WSL_DISTRO_NAME environment variable")

	InjectWslRootPath(t, func() ([]byte, error) {
		return []byte(`\\wsl.localhost\TEST_DISTRO\`), nil
	})

	InjectProStatusCmdOutput(t, func(ctx context.Context) ([]byte, error) {
		return []byte(`{"attached": false}`), nil
	})

	InjectOsRelease(t, func() ([]byte, error) {
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

	return &agentapi.DistroInfo{
		WslName:     "TEST_DISTRO",
		Id:          "ubuntu",
		VersionId:   "22.04",
		PrettyName:  "Ubuntu 22.04.1 LTS",
		ProAttached: false,
	}
}
