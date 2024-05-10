package daemon

import "github.com/canonical/ubuntu-pro-for-wsl/windows-agent/internal/daemon/testutils"

// WithWslSystemCmd sets the command to run to get the WSL networking mode.
func WithWslSystemCmd(cmd, cmdEnv []string) Option {
	return func(o *options) {
		o.wslSystemCmd = cmd
		o.wslCmdEnv = cmdEnv
	}
}

// WithMockedGetAdapterAddresses sets the function to use to get the adapter addresses from the mock object supplied.
func WithMockedGetAdapterAddresses(m testutils.MockIPConfig) Option {
	return func(o *options) {
		o.getAdaptersAddresses = func(family, flags uint32, reserved uintptr, adapterAddresses *ipAdapterAddresses, sizePointer *uint32) (errcode error) {
			return m.GetAdaptersAddresses(family, flags, reserved, (*testutils.IPAdapterAddresses)(adapterAddresses), sizePointer)
		}
	}
}
