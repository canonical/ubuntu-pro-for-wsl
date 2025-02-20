//go:build gowslmock

package distroinstall

import (
	"context"
	"errors"
	"fmt"

	wsl "github.com/ubuntu/gowsl"
)

// executableInstallCommand mocks running the command '$executable install --root --ui=none'.
// It intentionally fails for ubuntu0404.exe and ubuntu2404.exe, but it registers the latest, emulating the behaviour of a "modern" distro.
func executableInstallCommand(ctx context.Context, executable string) ([]byte, error) {
	switch executable {
	case "ubuntu0404.exe":
		return []byte("404: mock executable not found\n  + FullyQualifiedErrorId : CommandNotFoundException"), fmt.Errorf("exit status 1")
	case "ubuntu2204.exe":
		d := wsl.NewDistro(ctx, "Ubuntu-22.04")
		if err := d.Register("."); err != nil {
			return []byte(err.Error()), fmt.Errorf("exit status 1")
		}
		return []byte{}, nil
	case "ubuntu2404.exe":
		d := wsl.NewDistro(ctx, "Ubuntu-24.04")
		if err := d.Register("."); err != nil {
			return []byte(err.Error()), fmt.Errorf("exit status 1")
		}
		return []byte("mock executable not found: this is a tar-based distro\n  + FullyQualifiedErrorId : CommandNotFoundException"), fmt.Errorf("exit status 1")
	default:
		return []byte("mock supports only ubuntu2204.exe and ubuntu2404.exe"), fmt.Errorf("exit status 1")
	}
}

func addUserCommand(ctx context.Context, distro wsl.Distro, userName, userFullName string) (out []byte, err error) {
	if userName == "add_user_command_error" {
		return []byte("Mock error"), errors.New("exit status 1")
	}

	if userWasCreated(distro) {
		return []byte("adduser: The user already exists."), errors.New("exit status 1")
	}

	return []byte{}, markUserAsCreated(distro)
}

func addUserToGroupsCommand(ctx context.Context, distro wsl.Distro, userName string) ([]byte, error) {
	if userName == "add_user_to_groups_command_error" {
		return []byte("Mock error"), errors.New("exit status 1")
	}

	if !userWasCreated(distro) {
		return []byte("id: no such user"), errors.New("exit status 1")
	}

	return []byte{}, nil
}

func removePasswordCommand(ctx context.Context, distro wsl.Distro, userName string) ([]byte, error) {
	if userName == "remove_password_command_error" {
		return []byte("Mock error"), errors.New("exit status 1")
	}

	if !userWasCreated(distro) {
		return []byte("id: no such user"), errors.New("exit status 1")
	}

	return []byte{}, nil
}

func getUserIDCommand(ctx context.Context, distro wsl.Distro, userName string) ([]byte, error) {
	if userName == "get_user_id_command_error" {
		return []byte("Mock error"), errors.New("exit status 1")
	}

	if !userWasCreated(distro) {
		return []byte("id: no such user"), errors.New("exit status 1")
	}

	if userName == "get_user_id_command_bad_output" {
		return []byte("MockGetUserIdBadOutput"), nil
	}

	return []byte("1000"), nil
}

// markUserAsCreated is an ugly trick to store some persistent information.
// It highjacks the DriveMountingEnabled Configuration to signal
// userWasCreated whether it should return true or false.
func markUserAsCreated(d wsl.Distro) error {
	return d.DriveMountingEnabled(false)
}

// userWasCreated indicates wether the markUserAsCreated function was called.
func userWasCreated(d wsl.Distro) bool {
	c, err := d.GetConfiguration()
	if err != nil {
		panic(err.Error())
	}
	return !c.DriveMountingEnabled
}
