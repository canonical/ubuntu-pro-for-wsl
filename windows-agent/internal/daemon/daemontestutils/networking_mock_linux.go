package daemontestutils

import (
	"net"
	"syscall"

	"github.com/canonical/ubuntu-pro-for-wsl/common/testdetection"
)

// fillFromTemplate fills the ipAdapterAddresses structure with the values from the template.
func fillFromTemplate(template *MockIPAddrsTemplate, a, next *IPAdapterAddresses) {
	testdetection.MustBeTesting()

	a.FriendlyName = template.friendlyName
	a.Description = template.desc

	a.FirstUnicastAddress = template.ip
	a.Next = next
}

// IPAdapterAddresses redefines the wrapper type for the IP_ADAPTER_ADDRESSES structure for testing on Linux.
type IPAdapterAddresses struct {
	Next                *IPAdapterAddresses
	FriendlyName        string
	Description         string
	FirstUnicastAddress net.IP
}

// ERROR_BUFFER_OVERFLOW is the error returned by GetAdaptersAddresses when the buffer is too small.
//
//nolint:revive // Windows API constants are in shout case.
const ERROR_BUFFER_OVERFLOW = syscall.EOVERFLOW
