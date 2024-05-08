package daemon

import (
	"net"

	"golang.org/x/sys/windows"
)

// getWindowsAdaptersAddresses is a wrapper around windows.GetAdaptersAddresses that accepts our custom ipAdapterAddresses type so we can mock it in tests.
//
//nolint:unused // When linting with gowslmock built tag, this is flagged as unused because it's replaced with the mocked version. The rest of the file is still used.
func getWindowsAdaptersAddresses(family, flags uint32, reserved uintptr, adapterAddresses *ipAdapterAddresses, sizePointer *uint32) (errcode error) {
	return windows.GetAdaptersAddresses(family, flags, reserved, (*windows.IpAdapterAddresses)(adapterAddresses), sizePointer)
}

// ipAdapterAddresses is a wrapper around windows.IpAdapterAddresses to provide handy accessors and allow replacing with something else on Linux for testing.
type ipAdapterAddresses windows.IpAdapterAddresses

func (a *ipAdapterAddresses) next() *ipAdapterAddresses {
	return (*ipAdapterAddresses)(a.Next)
}

func (a *ipAdapterAddresses) friendlyName() string {
	return windows.UTF16PtrToString(a.FriendlyName)
}

func (a *ipAdapterAddresses) description() string {
	return windows.UTF16PtrToString(a.Description)
}

func (a *ipAdapterAddresses) ip() net.IP {
	return a.FirstUnicastAddress.Address.IP()
}

// ERROR_BUFFER_OVERFLOW is defined as a constant here so we can redefine it in tests on Linux.
//
//nolint:revive // Windows API constants are in shout case.
const ERROR_BUFFER_OVERFLOW = windows.ERROR_BUFFER_OVERFLOW
