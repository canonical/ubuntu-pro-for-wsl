package daemon

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"math"
	"net"
	"os/exec"
	"reflect"
	"strings"

	log "github.com/canonical/ubuntu-pro-for-wsl/common/grpc/logstreamer"
	"github.com/ubuntu/decorate"
)

type options struct {
	wslCmd               []string
	wslCmdEnv            []string
	getAdaptersAddresses getAdaptersAddressesFunc
}

var defaultOptions = options{
	wslCmd:               []string{"wsl.exe"},
	getAdaptersAddresses: getWindowsAdaptersAddresses,
}

// Option represents an optional function to override getWslIP default values.
type Option func(*options)

type getAdaptersAddressesFunc func(family uint32, flags uint32, reserved uintptr, adapterAddresses *ipAdapterAddresses, sizePointer *uint32) (errcode error)

// getWslIP returns the loopback address if the networking mode is mirrored or iterates over the network adapters to find the IP address of the WSL one.
func getWslIP(ctx context.Context, args ...Option) (ip net.IP, err error) {
	defer decorate.OnError(&err, "could not determine WSL IP address: ")

	opts := defaultOptions
	for _, arg := range args {
		arg(&opts)
	}

	mode, err := networkingMode(ctx, opts.wslCmd, opts.wslCmdEnv)
	if err != nil {
		// NAT is assumed because it's the default networking mode for WSL as of 2024.
		log.Warningf(ctx, "could not determine if WSL network is mirrored (assuming NAT): %v", err)
		mode = "nat"
	}

	switch mode {
	case "mirrored":
		return net.IPv4(127, 0, 0, 1), nil
	case "nat":
		return findWslAdapterIP(opts)
	default:
		return nil, fmt.Errorf("unknown networking mode: %s", mode)
	}
}

// findWslAdapterIP iterates over the network adapters to find the IP address of the WSL one.
func findWslAdapterIP(opts options) (net.IP, error) {
	head, err := getAddrList(opts)
	if err != nil {
		return nil, err
	}

	// Filter the adapters by description.
	for node := head; node != nil; node = node.next() {
		// The adapter description could also be "Hyper-V Virtual Ethernet Adapter #No"
		if !strings.Contains(node.description(), "Hyper-V Virtual Ethernet Adapter") {
			continue
		}
		// Desambiguates the adapters by friendly name.
		// Tested with WSL versions 1.2.5 and 2.1.5 on machines with and without Hyper-V Manager:
		// the friendly name may change between versions and whether Hyper-V is enabled on the
		// Windows machine or not, but it will contain the string "vEthernet (WSL".
		if !strings.Contains(node.friendlyName(), "vEthernet (WSL") {
			continue
		}

		return node.ip(), nil
	}

	return nil, fmt.Errorf("could not find WSL adapter")
}

// networkingMode detects whether the WSL network is mirrored or not.
func networkingMode(ctx context.Context, wslCmd, cmdEnv []string) (string, error) {
	// It does so by launching the system distribution (wsl --system).
	name := wslCmd[0]
	args := append(wslCmd[1:], "--system", "wslinfo", "--networking-mode", "-n")
	//nolint:gosec //Subprocess is launched from a variable to be testable.
	cmd := exec.CommandContext(ctx, name, args...)

	var stdout, stderr bytes.Buffer

	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	cmd.Env = cmdEnv

	err := cmd.Run()
	out := bytes.TrimSpace(stdout.Bytes())
	if err != nil {
		return "", fmt.Errorf(
			"%s: error: %v.\n    Stdout: %s\n    Stderr: %s",
			cmd.Path,
			err,
			out,
			stderr.String(),
		)
	}

	return strings.Split(string(out), "\n")[0], nil
}

// getAddrList returns the head of a linked list of network adapters addresses.
func getAddrList(opts options) (head *ipAdapterAddresses, err error) {
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
		family uint32 = 2 // windows.AF_INET
		flags  uint32 = GAA_FLAG_SKIP_ANYCAST | GAA_FLAG_SKIP_MULTICAST | GAA_FLAG_SKIP_DNS_SERVER
	)

	// We need a typed buffer rather than []byte because we don't want the GC to move
	// the buffer around while we're using it, invalidating the NEXT pointers.
	var buff buffer[ipAdapterAddresses]

	// Win32 API docs recommend a buff size of 15KB.
	size := 15 * kilobyte
	for range 10 {
		size, err = buff.resizeBytes(size)
		if err != nil {
			return nil, err
		}
		err = opts.getAdaptersAddresses(family, flags, 0, &buff.data[0], &size)
		if errors.Is(err, ERROR_BUFFER_OVERFLOW) {
			// Buffer too small, try again with the returned size.
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

// ResizeBytes resizes the buffer to the given number of bytes, rounded UP to fit an integer element size.
func (b *buffer[T]) resizeBytes(n uint32) (uint32, error) {
	var t T
	n64 := uint64(n)
	sizeOf := uint64(reflect.TypeOf(t).Size())

	newLen := n64 / sizeOf
	if n64%sizeOf != 0 {
		newLen++
	}

	// the sizes the Win32 API GetAdaptersAddresses works with are uint32, thus we cannot allocate
	// more than MaxUint32 bytes after all.
	newSize := newLen * sizeOf
	if newSize >= math.MaxUint32 {
		return 0, errors.New("buffer allocated size limit reached")
	}

	if newLen > uint64(len(b.data)) {
		b.data = make([]T, newLen)
		// Since make() guarantees len(b.data) == newLen, there is no need to recompute it.
	}

	//nolint:gosec //uint64 -> uint32 conversion is safe because we checked that newSize < MaxUint32.
	return uint32(newSize), nil
}

// ptr returns a pointer to the start of the buffer.
func (b *buffer[T]) ptr() *T {
	if len(b.data) == 0 {
		return nil
	}
	return &b.data[0]
}
