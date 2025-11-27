//go:build gowslmock

// Package touchdistro exists to provide multiple, mockable implementations
// for the actions of touching a distro, i.e. sending a short-lived command so
// as to wake it up, and waiting for distro initialisation with cloud-init.
package touchdistro

import (
	"context"
	"errors"
	"fmt"
	"strings"

	wsl "github.com/ubuntu/gowsl"
)

// Touch sends a "exit 0" command to a distro in order to wake it up.
// It returns wslDistroNotFoundError when the distroName contains the
// unregister magic word, to ease testing.
func Touch(ctx context.Context, distroName string) error {
	if strings.Contains(distroName, "unregistered") {
		return &wslDistroNotFoundError{errors.New(distroName)}
	}
	d := wsl.NewDistro(ctx, distroName)

	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	if err := d.Shell(wsl.WithCommand("exit 0")); err != nil {
		return fmt.Errorf("could not run 'exit 0': %v", err)
	}

	return nil
}

// WaitForCloudInit sends a "exit 0" command to a distro because tests are not really interested in details of a cloud-init run.
func WaitForCloudInit(ctx context.Context, distroName string) error {
	d := wsl.NewDistro(ctx, distroName)

	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	if err := d.Shell(wsl.WithCommand("exit 0")); err != nil {
		return fmt.Errorf("could not pretend to run 'cloud-init': %v", err)
	}

	return nil
}
