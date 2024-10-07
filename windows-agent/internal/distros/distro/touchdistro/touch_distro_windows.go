//go:build !gowslmock

// Package touchdistro exists to provide multiple, mockable implementations
// for the actions of touching a distro, i.e. sending a short-lived command so
// as to wake it up, and waiting for distro initialisation with cloud-init.
package touchdistro

import (
	"context"
	"fmt"
	"os/exec"
	"syscall"
)

// Touch sends a "exit 0" command to a distro in order to wake it up.
func Touch(ctx context.Context, distroName string) error {
	cmd := wslCmd(ctx, distroName, "exit", "0")
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("could not run 'exit 0': %v. Output: %s", err, out)
	}

	return nil
}

// WaitForCloudInit blocks the caller until cloud-init has finished initialising the distro.
func WaitForCloudInit(ctx context.Context, distroName string) error {
	cmd := wslCmd(ctx, distroName, "cloud-init", "status", "--wait")
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("could not run 'cloud-init': %v. Output: %s", err, out)
	}

	return nil
}

func wslCmd(ctx context.Context, distroName string, args ...string) *exec.Cmd {
	// https://learn.microsoft.com/en-us/windows/win32/procthread/process-creation-flags
	//
	// CREATE_NO_WINDOW:
	// The process is a console application that is being run without
	// a console window. Therefore, the console handle for the
	// application is not set.
	const createNoWindow = 0x08000000

	cmd := exec.CommandContext(ctx, "wsl.exe", append([]string{"-d", distroName, "--"}, args...)...)
	cmd.SysProcAttr = &syscall.SysProcAttr{
		HideWindow:    true,
		CreationFlags: createNoWindow,
	}

	return cmd
}
