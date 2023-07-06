//go:build !gowslmock

package distroinstall

import (
	"context"

	wsl "github.com/ubuntu/gowsl"
)

func executableInstallCommand(ctx context.Context, executable string) (out []byte, err error) {
	panic("executableInstallCommand: this function can only be run on Windows")
}

func addUserCommand(ctx context.Context, distro wsl.Distro, userName, userFullName string) (out []byte, err error) {
	panic("addUserCommand: this function can only be run on Windows")
}

func addUserToGroupsCommand(ctx context.Context, distro wsl.Distro, userName string) ([]byte, error) {
	panic("addUserToGroupsCommand: this function can only be run on Windows")
}

func removePasswordCommand(ctx context.Context, distro wsl.Distro, userName string) ([]byte, error) {
	panic("removePasswordCommand: this function can only be run on Windows")
}

func getUserIDCommand(ctx context.Context, distro wsl.Distro, userName string) ([]byte, error) {
	panic("getUserIdCommand: this function can only be run on Windows")
}
