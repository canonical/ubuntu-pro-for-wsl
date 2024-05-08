package testutils

import (
	"net"
	"syscall"
	"unsafe"

	"github.com/canonical/ubuntu-pro-for-wsl/common/testdetection"
	"golang.org/x/sys/windows"
)

// fillFromTemplate fills the ipAdapterAddresses struct with the values from the mockIPAddrsTemplate struct for testing purposes.
func fillFromTemplate(template *MockIPAddrsTemplate, a, next *IPAdapterAddresses) {
	testdetection.MustBeTesting()

	// To constrain complexity we only fill the pieces of data we need in the actual code, skipping a lot of details and fields.
	a.FriendlyName = windows.StringToUTF16Ptr(template.friendlyName)
	a.Description = windows.StringToUTF16Ptr(template.desc)

	ip := ipToRawSockaddrAny(template.ip)

	a.FirstUnicastAddress = &windows.IpAdapterUnicastAddress{
		Address: windows.SocketAddress{
			Sockaddr:       ip,
			SockaddrLength: int32(unsafe.Sizeof(windows.RawSockaddrInet4{})),
		},
	}

	a.Next = (*windows.IpAdapterAddresses)(next)
}

// ipToRawSockaddrAny is a helper function that converts an assumed valid net.IP (IPv4) to *syscall.RawSockaddrAny.
func ipToRawSockaddrAny(ip net.IP) *syscall.RawSockaddrAny {
	testdetection.MustBeTesting()

	ip4 := ip.To4()
	if ip4 == nil {
		return nil
	}

	sa := new(syscall.RawSockaddrInet4)
	sa.Family = syscall.AF_INET
	copy(sa.Addr[:], ip4) // ip4 is already a 4-byte slice

	//nolint:gosec // Unsafe is required to manipulate pointers at the Win32 API level, only used in tests.
	return (*syscall.RawSockaddrAny)(unsafe.Pointer(sa))
}

// IPAdapterAddresses is a type alias for windows.IpAdapterAddresses.
type IPAdapterAddresses windows.IpAdapterAddresses

// ERROR_BUFFER_OVERFLOW is defined as a constant here so we can redefine it in tests on Linux.
//
//nolint:revive // Windows API constants are in shout case.
const ERROR_BUFFER_OVERFLOW = windows.ERROR_BUFFER_OVERFLOW
