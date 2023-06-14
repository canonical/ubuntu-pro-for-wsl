//go:build !gowslmock

// Package touchdistro exists to provide multiple, mockable implementations
// for the action of touching a distro, i.e. sending a short-lived command so
// as to wake it up.
package touchdistro

import (
	"context"
	"fmt"
	"os/exec"
	"syscall"
)

// Touch sends a "exit 0" command to a distro in order to wake it up.
func Touch(ctx context.Context, distroName string) error {
	// https://learn.microsoft.com/en-us/windows/win32/procthread/process-creation-flags
	//
	// CREATE_NO_WINDOW:
	// The process is a console application that is being run without
	// a console window. Therefore, the console handle for the
	// application is not set.
	const createNoWindow = 0x08000000

	cmd := exec.CommandContext(ctx, "wsl.exe", "-d", distroName, "--", "exit", "0")
	cmd.SysProcAttr = &syscall.SysProcAttr{
		HideWindow:    true,
		CreationFlags: createNoWindow,
	}

	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("could not run 'exit 0': %v. Output: %s", err, out)
	}

	return nil
}
