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

	return &agentapi.DistroInfo{
		WslName:     "TEST_DISTRO",
		Id:          "ubuntu",
		VersionId:   "22.04",
		PrettyName:  "Ubuntu 22.04.1 LTS",
		ProAttached: false,
	}
}
