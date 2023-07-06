//go:build gowslmock

package distroinstall

import (
	"context"
	"errors"
	"fmt"

	wsl "github.com/ubuntu/gowsl"
)

const (
	userCauseError = "mockerror"
)

// executableInstallCommand mocks running the command '$executable install --root --ui=none'
func executableInstallCommand(ctx context.Context, executable string) ([]byte, error) {
	if executable != "ubuntu2204.exe" {
		return []byte("mock supports only ubuntu2204.exe"), fmt.Errorf("exit status 1")
	}

	d := wsl.NewDistro(ctx, "Ubuntu-22.04")
	if err := d.Register("."); err != nil {
		return []byte(err.Error()), fmt.Errorf("exit status 1")
	}

	return []byte{}, nil
}

func addUserCommand(ctx context.Context, distro wsl.Distro, uid uint32, userName, userFullName string) (out []byte, err error) {
	if userName == userCauseError {
		return []byte("Mock error"), errors.New("exit status 1")
	}

	return []byte{}, nil
}
