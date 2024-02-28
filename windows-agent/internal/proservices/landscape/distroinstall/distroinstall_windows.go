//go:build !gowslmock

package distroinstall

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"syscall"

	wsl "github.com/ubuntu/gowsl"
)

// https://learn.microsoft.com/en-us/windows/win32/procthread/process-creation-flags
//
// CREATE_NO_WINDOW:
// The process is a console application that is being run without
// a console window. Therefore, the console handle for the
// application is not set.
const createNoWindow = 0x08000000

func executableInstallCommand(ctx context.Context, executable string) (out []byte, err error) {
	// We need to use powershell because the Appx executable is not in the path
	cmd := exec.CommandContext(ctx, "powershell.exe",
		"-NoLogo", "-NoProfile", "-NonInteractive", "-Command",
		executable, "install", "--root")
	cmd.Env = append(os.Environ(), "WSL_UTF8=1")
	cmd.SysProcAttr = &syscall.SysProcAttr{
		HideWindow:    true,
		CreationFlags: createNoWindow,
	}

	return cmd.CombinedOutput()
}

func addUserCommand(ctx context.Context, distro wsl.Distro, userName, userFullName string) (out []byte, err error) {
	cmd := wslCommand(ctx, distro,
		"adduser", userName,
		fmt.Sprintf("--gecos=%q", userFullName),
		"--quiet")
	cmd.SysProcAttr = &syscall.SysProcAttr{
		HideWindow:    true,
		CreationFlags: createNoWindow,
	}

	return cmd.CombinedOutput()
}

func addUserToGroupsCommand(ctx context.Context, distro wsl.Distro, userName string) ([]byte, error) {
	cmd := wslCommand(ctx, distro, "usermod", "-aG", "adm,dialout,cdrom,floppy,sudo,audio,dip,video,plugdev,netdev", userName)
	cmd.SysProcAttr = &syscall.SysProcAttr{
		HideWindow:    true,
		CreationFlags: createNoWindow,
	}

	return cmd.CombinedOutput()
}

func removePasswordCommand(ctx context.Context, distro wsl.Distro, userName string) ([]byte, error) {
	cmd := wslCommand(ctx, distro, "passwd", "-d", userName)
	return cmd.CombinedOutput()
}

func getUserIDCommand(ctx context.Context, distro wsl.Distro, userName string) ([]byte, error) {
	cmd := wslCommand(ctx, distro, "id", "-u", userName)
	return cmd.CombinedOutput()
}

// wslCommand creates a Cmd at the selected distro in a way that won't cause a console to start.
func wslCommand(ctx context.Context, distro wsl.Distro, path string, args ...string) *exec.Cmd {
	args = append([]string{"-u", "root", "-d", distro.Name(), "--", path}, args...)

	cmd := exec.CommandContext(ctx, "wsl.exe", args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{
		HideWindow:    true,
		CreationFlags: createNoWindow,
	}

	return cmd
}
