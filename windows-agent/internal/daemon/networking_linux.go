package daemon

import (
	"net"
	"syscall"

	"github.com/canonical/ubuntu-pro-for-wsl/common/testdetection"
)

// ERROR_BUFFER_OVERFLOW is the error returned by GetAdaptersAddresses when the buffer is too small.
//
//nolint:revive // Windows API constants are in shout case.
const ERROR_BUFFER_OVERFLOW = syscall.EOVERFLOW

// ipAdapterAddresses redefines the wrapper type for the IP_ADAPTER_ADDRESSES structure for testing on Linux.
type ipAdapterAddresses struct {
	Next                *ipAdapterAddresses
	FriendlyName        string
	Description         string
	FirstUnicastAddress net.IP
}

func (a *ipAdapterAddresses) next() *ipAdapterAddresses {
	return a.Next
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

// fillFromTemplate fills the ipAdapterAddresses structure with the values from the template.
func (a *ipAdapterAddresses) fillFromTemplate(template *mockIPAddrsTemplate, next *ipAdapterAddresses) {
	testdetection.MustBeTesting()

	a.FriendlyName = template.friendlyName
	a.Description = template.desc

	a.FirstUnicastAddress = template.ip
	a.Next = next
}
