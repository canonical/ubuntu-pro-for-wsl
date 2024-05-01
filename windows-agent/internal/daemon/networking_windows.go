package daemon

import (
	"net"
	"syscall"
	"unsafe"

	"github.com/canonical/ubuntu-pro-for-wsl/common/testdetection"
	"golang.org/x/sys/windows"
)

func init() {
	defaultOptions = options{
		wslSystemCmd:         []string{"wsl.exe", "--system", "wslinfo", "--networking-mode", "-n"},
		getAdaptersAddresses: getWindowsAdaptersAddresses,
	}
}

// getWindowsAdaptersAddresses is a wrapper around windows.GetAdaptersAddresses that accepts our custom ipAdapterAddresses type so we can mock it in tests.
//
//nolint:unused // When linting with gowslmock built tag, this is flagged as unused because it's replaced with the mocked version. The rest of the file is still used.
func getWindowsAdaptersAddresses(family uint32, flags uint32, reserved uintptr, adapterAddresses *ipAdapterAddresses, sizePointer *uint32) (errcode error) {
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

// fillFromTemplate fills the ipAdapterAddresses struct with the values from the mockIPAddrsTemplate struct for testing purposes.
func (a *ipAdapterAddresses) fillFromTemplate(template *mockIPAddrsTemplate, next *ipAdapterAddresses) {
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

// ERROR_BUFFER_OVERFLOW is defined as a constant here so we can redefine it in tests on Linux.
//
//nolint:revive // Windows API constants are in shout case.
const ERROR_BUFFER_OVERFLOW = windows.ERROR_BUFFER_OVERFLOW
