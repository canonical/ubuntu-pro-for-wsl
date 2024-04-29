package daemon

import (
	"context"
	"errors"
	"net"
	"os/exec"
	"reflect"

	"github.com/ubuntu/decorate"
	"golang.org/x/sys/windows"
)

type winIPAdapterAddresses struct {
	node *windows.IpAdapterAddresses
}

func (n *winIPAdapterAddresses) Next() ipAdapterAddresses {
	if n.node.Next == nil {
		return nil
	}
	return &winIPAdapterAddresses{n.node.Next}
}

func (n *winIPAdapterAddresses) Description() string {
	return windows.UTF16PtrToString(n.node.Description)
}

func (n *winIPAdapterAddresses) FriendlyName() string {
	return windows.UTF16PtrToString(n.node.FriendlyName)
}

func (n *winIPAdapterAddresses) IP() net.IP {
	return n.node.FirstUnicastAddress.Address.IP()
}

// getAdaptersAddresses returns the head of a linked list of network adapters.
func (i ipConfig) getAdaptersAddresses() (head ipAdapterAddresses, err error) {
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
		return &winIPAdapterAddresses{buff.ptr()}, nil
	}

	// We tried 10 times and the buffer is still too small: give up.
	return nil, errors.New("iteration limit reached")
}

type ipConfig struct{}

func newIPConfig() ipConfig {
	return ipConfig{}
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

type wslSystem struct{}

func newWslSystem() wslSystem {
	return wslSystem{}
}

// Command provides an *exec.Command configured to run inside the always present system distro, useful when we cannot guaratee the presence of any other distro instance.
func (wsl wslSystem) Command(ctx context.Context, name string, arg ...string) *exec.Cmd {
	args := append([]string{"--system", name}, arg...)
	//nolint: gosec // Subprocess launched with variable (gosec) intentionally so we can mock it.
	return exec.CommandContext(ctx, "wsl", args...)
}
