//go:build !gowslmock

package distroinstall

import (
	"context"
	"fmt"
	"regexp"
)

// InstallFromExecutable finds the executable associated with the specified distro and installs it.
func InstallFromExecutable(ctx context.Context, distroName string) error {
	panic("InstallFromExecutable: this function can only be run on Windows")
}

func addUserCommand(ctx context.Context, distro gowsl.Distro, uid uint32, userName, userFullName string) (out []byte, err error) {
	panic("addUserCommand: this function can only be run on Windows")
}
