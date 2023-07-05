//go:build !gowslmock

package distroinstall

import (
	"context"
	"fmt"
	"os/exec"
	"syscall"

	"github.com/ubuntu/gowsl"
)

// https://learn.microsoft.com/en-us/windows/win32/procthread/process-creation-flags
//
// CREATE_NO_WINDOW:
// The process is a console application that is being run without
// a console window. Therefore, the console handle for the
// application is not set.
const createNoWindow = 0x08000000

// InstallFromExecutable finds the executable associated with the specified distro and installs it.
func executableInstallCommand(ctx context.Context, executable string) (out []byte, err error) {
	// We need to use powershell because the Appx executable is not in the path
	//nolint:gosec // The executable is validated by the caller.
	cmd := exec.CommandContext(ctx, "powershell.exe",
		"-NoLogo", "-NoProfile", "-NonInteractive", "-Command",
		executable, "install", "--root", "--ui=none")
	cmd.SysProcAttr = &syscall.SysProcAttr{
		HideWindow:    true,
		CreationFlags: createNoWindow,
	}

	return cmd.CombinedOutput()
}

func addUserCommand(ctx context.Context, distro gowsl.Distro, uid uint32, userName, userFullName string) (out []byte, err error) {
	//nolint:gosec // This is a private function, and the caller verifies all inputs.
	cmd := exec.CommandContext(ctx, "wsl.exe", "-d", distro.Name(), "--",
		"adduser", userName,
		fmt.Sprintf("--uid=%d", uid),
		fmt.Sprintf("--gecos=%q", userFullName),
		"--disabled-password",
		"--quiet")
	cmd.SysProcAttr = &syscall.SysProcAttr{
		HideWindow:    true,
		CreationFlags: createNoWindow,
	}

	return cmd.CombinedOutput()
}
