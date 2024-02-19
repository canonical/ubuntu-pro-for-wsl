//go:build !gowslmock

package distroinstall

import (
	"context"
	"os/exec"
	"syscall"
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
