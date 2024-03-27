// Package distroinstall exists to implement various utilities used by landscape that need to be mocked
// in tests. As such, the real implementations are located in the _windows files, and the mocks in the
// _gowslmock files. Use build tag gowslmock to enable the latter.
package distroinstall

import (
	"context"
	"fmt"

	"github.com/canonical/ubuntu-pro-for-wsl/common"
	"github.com/ubuntu/gowsl"
)

// InstallFromExecutable finds the executable associated with the specified distro and installs it.
func InstallFromExecutable(ctx context.Context, d gowsl.Distro) error {
	executable, err := common.WSLLauncher(d.Name())
	if err != nil {
		return err
	}

	if out, err := executableInstallCommand(ctx, executable); err != nil {
		return fmt.Errorf("could not run launcher: %v. %s", err, out)
	}

	return err
}
