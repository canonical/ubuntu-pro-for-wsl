package daemon

import (
	"context"
	"os"

	"github.com/canonical/ubuntu-pro-for-wsl/windows-agent/internal/daemon/daemontestutils"
)

// WithWslNetworkingMode sets the output of the mock command to run to get the WSL networking mode.
func WithWslNetworkingMode(netmode string) Option {
	return func(o *options) {
		o.wslCmd = []string{
			os.Args[0],
			"-test.run",
			"TestWithWslSystemMock",
			"--",
			netmode,
		}
		o.wslCmdEnv = []string{"GO_WANT_HELPER_PROCESS=1"}
	}
}

// WithMockedGetAdapterAddresses sets the function to use to get the adapter addresses from the mock object supplied.
func WithMockedGetAdapterAddresses(m daemontestutils.MockIPConfig) Option {
	return func(o *options) {
		o.getAdaptersAddresses = func(family, flags uint32, reserved uintptr, adapterAddresses *ipAdapterAddresses, sizePointer *uint32) (errcode error) {
			return m.GetAdaptersAddresses(family, flags, reserved, (*daemontestutils.IPAdapterAddresses)(adapterAddresses), sizePointer)
		}
	}
}

// Restart exposes the private restart method for testing purposes.
func (d *Daemon) Restart(ctx context.Context) {
	d.restart(ctx)
}
