//go:build !gowslmock

package distroinstall

import (
	"context"
)

func executableInstallCommand(ctx context.Context, executable string) (out []byte, err error) {
	panic("executableInstallCommand: this function can only be run on Windows")
}
