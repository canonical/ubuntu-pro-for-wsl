//go:build !gowslmock

package daemon

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os/exec"
	"reflect"
	"strings"

	log "github.com/canonical/ubuntu-pro-for-wsl/common/grpc/logstreamer"
	"github.com/ubuntu/decorate"
	"golang.org/x/sys/windows"
)

//nolint:unused // Does nothing; it exists so we can compile the tests without mocks.
var wslIPErr bool

// getWslIP returns the loopback address if the networking mode is mirrored or iterates over the network adapters to find the IP address of the WSL one.
func getWslIP() (net.IP, error) {
	isMirrored, err := networkIsMirrored()
	if err != nil {
		log.Warningf(context.Background(), "could not determine if WSL network is mirrored (assuming NAT): %v", err)
	}
	if isMirrored {
		return net.IPv4(127, 0, 0, 1), nil
	}

	const targetDesc = "Hyper-V Virtual Ethernet Adapter"
	const vEthernetName = "vEthernet (WSL"

	head, err := getAdaptersAddresses()
	if err != nil {
		return nil, err
	}

	// Filter the adapters by description.
	var candidates []*windows.IpAdapterAddresses
	for node := head; node != nil; node = node.Next {
		desc := windows.UTF16PtrToString(node.Description)
		// The adapter description could be "Hyper-V Virtual Ethernet Adapter #No"
		if !strings.Contains(desc, targetDesc) {
			continue
		}

		candidates = append(candidates, node)
	}

	if len(candidates) == 1 {
		return candidates[0].FirstUnicastAddress.Address.IP(), nil
	}

	// Desambiguates the adapters by friendly name.
	for _, node := range candidates {
		if !strings.Contains(windows.UTF16PtrToString(node.FriendlyName), vEthernetName) {
			continue
		}

		return node.FirstUnicastAddress.Address.IP(), nil
	}

	return nil, fmt.Errorf("could not find WSL adapter")
}

// networkIsMirrored detects whether the WSL network is mirrored or not.
func networkIsMirrored() (bool, error) {
	// It does so by launching the system distribution.
	cmd := exec.CommandContext(context.Background(), "wsl", "--system", "wslinfo", "--networking-mode", "-n")

	out, err := cmd.Output()
	if err != nil {
		return false, fmt.Errorf("could not get networking mode: %w\n%s", err, string(out))
	}
	return string(out) == "mirrored", nil
}

// getAdaptersAddresses returns the head of a linked list of network adapters.
func getAdaptersAddresses() (head *windows.IpAdapterAddresses, err error) {
	defer decorate.OnError(&err, "could not get network adapter addresses")

	// This function is a wrapper around the Windows API GetAdaptersAddresses.
	// https://learn.microsoft.com/en-us/windows/win32/api/iphlpapi/nf-iphlpapi-getadaptersaddresses
	//
	// This function takes in a buffer and fills it with a linked list.

	// Flags from the Windows API.
	//nolint:revive // Windows API constants are in shout case.
	const (
		GAA_FLAG_SKIP_ANYCAST    uint32 = 0x0002
		GAA_FLAG_SKIP_MULTICAST  uint32 = 0x0004
		GAA_FLAG_SKIP_DNS_SERVER uint32 = 0x0008
	)

	// Return only IPv4 unicast addresses
	const (
		family uint32 = windows.AF_INET
		flags  uint32 = GAA_FLAG_SKIP_ANYCAST | GAA_FLAG_SKIP_MULTICAST | GAA_FLAG_SKIP_DNS_SERVER
	)

	// We need a typed buffer rather than []byte because we don't want the GC to move
	// the buffer around while we're using it, invalidating the NEXT pointers.
	var buff buffer[windows.IpAdapterAddresses]

	// Win32 API docs recommend a buff size of 15KB.
	buff.resizeBytes(15 * kilobyte)

	for range 10 {
		size := buff.byteCount()
		err := windows.GetAdaptersAddresses(family, flags, 0, &buff.data[0], &size)
		if errors.Is(err, windows.ERROR_BUFFER_OVERFLOW) {
			// Buffer too small, try again with the returned size.
			buff.resizeBytes(size)
			continue
		}
		if err != nil {
			return nil, err
		}

		// The buffer is filled with the linked list of adapters, with the first element being the head.
		// We return a pointer to the start of the buffer.
		return buff.ptr(), nil
	}

	// We tried 10 times and the buffer is still too small: give up.
	return nil, errors.New("iteration limit reached")
}

// Constants for byte size conversion.
const kilobyte uint32 = 1024

// buffer is a type that allows resizing a slice of any type to a given number of bytes.
type buffer[T any] struct {
	data []T
}

// byteCount returns the number of bytes in the buffer.
func (b buffer[T]) byteCount() uint32 {
	var t T
	sizeOf := uint32(reflect.TypeOf(t).Size())
	n := uint32(len(b.data))
	return n * sizeOf
}

// ResizeBytes resizes the buffer to the given number of bytes, rounded UP to fit an integer element size.
func (b *buffer[T]) resizeBytes(n uint32) {
	var t T
	sizeOf := uint32(reflect.TypeOf(t).Size())

	newLen := int(n / sizeOf)
	if n%sizeOf != 0 {
		newLen++
	}

	if newLen > len(b.data) {
		b.data = make([]T, newLen)
	}
}

// ptr returns a pointer to the start of the buffer.
func (b *buffer[T]) ptr() *T {
	if len(b.data) == 0 {
		return nil
	}
	return &b.data[0]
}
