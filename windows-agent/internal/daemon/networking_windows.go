//go:build !gowslmock

package daemon

import (
	"errors"
	"fmt"
	"net"
	"unsafe"

	"golang.org/x/sys/windows"
)

// Does nothing, exists so we can compile the tests without mocks.
var wslIPErr bool

func getWslIP() (net.IP, error) {
	const targetName = "Hyper-V Virtual Ethernet Adapter"

	head, err := getAdaptersAddresses()
	if err != nil {
		return nil, err
	}

	for addr := head; addr != nil; addr = addr.Next {
		desc := safeUTF16ToString(addr.Description, len(targetName)+1)
		if desc != targetName {
			continue
		}

		return addr.FirstUnicastAddress.Address.IP(), nil
	}

	return nil, fmt.Errorf("could not find WSL adapter")
}

// getAdaptersAddresses returns the head of a linked list of network adapters.
func getAdaptersAddresses() (*windows.IpAdapterAddresses, error) {
	// Flags from the Windows API.
	// https://learn.microsoft.com/en-us/windows/win32/api/iphlpapi/nf-iphlpapi-getadaptersaddresses
	//
	//nolint:revive // Windows API constants are in shout case.
	const (
		GAA_FLAG_SKIP_ANYCAST       uint32 = 0x0002
		GAA_FLAG_SKIP_MULTICAST     uint32 = 0x0004
		GAA_FLAG_SKIP_DNS_SERVER    uint32 = 0x0008
		GAA_FLAG_SKIP_FRIENDLY_NAME uint32 = 0x0010
	)

	// Return only IPv4 unicast addresses
	const (
		family uint32 = windows.AF_INET
		flags  uint32 = GAA_FLAG_SKIP_ANYCAST | GAA_FLAG_SKIP_MULTICAST | GAA_FLAG_SKIP_DNS_SERVER | GAA_FLAG_SKIP_FRIENDLY_NAME
	)

	buf := make([]windows.IpAdapterAddresses, 1)
	for i := 0; i < 100; i++ {
		size := uint32(len(buf))

		err := windows.GetAdaptersAddresses(family, flags, 0, &buf[0], &size)
		if errors.Is(err, windows.ERROR_BUFFER_OVERFLOW) {
			// Buffer too small, try again with the returned size.
			buf = make([]windows.IpAdapterAddresses, size)
			continue
		}
		if err != nil {
			return nil, err
		}

		break
	}

	// Returning the buffer would be confusing to the caller, as it is a fake slice.
	// Only the first element is valid, accessing any other element causes a panic.
	return &buf[0], nil
}

// safeUTF16ToString is equivalent to windows.UTF16ToString, but it takes a maximum length
// to avoid reading past the end of the buffer.
func safeUTF16ToString(ptr *uint16, maxLen int) string {
	//nolint:gosec // This is safe because:
	// 1. This slice does not escape the function.
	// 2. windows.UTF16ToString checks for null-termination.
	s := unsafe.Slice(ptr, maxLen)

	return windows.UTF16ToString(s)
}
