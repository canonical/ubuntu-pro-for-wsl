//go:build !gowslmock

// Package touchdistro exists to provide multiple, mockable implementations
// for the actions of touching a distro, i.e. sending a short-lived command so
// as to wake it up, and waiting for distro initialisation with cloud-init.
package touchdistro

import (
	"context"
)

// Touch is a stub function panics. Use the gowslmock in order to use it in Linux.
func Touch(ctx context.Context, distroName string) error {
	panic("Touch: this function can only be run on Windows")
}

// WaitForCloudInit is a stub function panics. Use the gowslmock in order to use it in Linux.
func WaitForCloudInit(ctx context.Context, distroName string) error {
	panic("WaitForCloudInit: this function can only be run on Windows")
}
