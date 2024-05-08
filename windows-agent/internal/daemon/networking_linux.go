package daemon

import (
	"net"

	"github.com/canonical/ubuntu-pro-for-wsl/windows-agent/internal/daemon/testutils"
)

// ERROR_BUFFER_OVERFLOW is the error returned by GetAdaptersAddresses when the buffer is too small.
//
//nolint:revive // Windows API constants are in shout case.
const ERROR_BUFFER_OVERFLOW = testutils.ERROR_BUFFER_OVERFLOW

// ipAdapterAddresses redefines the wrapper type for the IP_ADAPTER_ADDRESSES structure for testing on Linux.
type ipAdapterAddresses testutils.IPAdapterAddresses

func (a *ipAdapterAddresses) next() *ipAdapterAddresses {
	return (*ipAdapterAddresses)(a.Next)
}

func (a *ipAdapterAddresses) friendlyName() string {
	return a.FriendlyName
}

func (a *ipAdapterAddresses) description() string {
	return a.Description
}

func (a *ipAdapterAddresses) ip() net.IP {
	return a.FirstUnicastAddress
}

// getWindowsAdaptersAddresses is a fake wrapper that panics if invoked, which only exists to satisfy setting `defaultOptions` in networking.go with a Linux "implementation".
func getWindowsAdaptersAddresses(family, flags uint32, reserved uintptr, adapterAddresses *ipAdapterAddresses, sizePointer *uint32) (errcode error) {
	panic("getWindowsAdaptersAddresses is not implemented on Linux without a mock")
}
