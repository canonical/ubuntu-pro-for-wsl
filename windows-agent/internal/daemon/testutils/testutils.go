// Package testutils exports test helpers to be used in other packages that need to change internal behaviors of the daemon.
package testutils

import (
	"fmt"
	"os"
	"slices"
	"strings"
	"testing"
	//nolint:revive,nolintlint // needed for go:linkname, but only used in tests. nolintlint as false positive then.
	_ "unsafe"

	"github.com/canonical/ubuntu-pro-for-wsl/common/testdetection"
)

// MockWslSystemCmd mocks commands running inside the WSL system distro.
// Add it to your package_test with:
//
//	func TestWithWslSystemMock(t *testing.T) { daemontests.MockWslSystemCmd(t) }
//
//nolint:thelper // This is a faux test used to mock commands running via `wsl --system`
func MockWslSystemCmd(t *testing.T) {
	testdetection.MustBeTesting()

	const errorUsage = `
wslinfo usage:
	--networking-mode
		Display current networking mode.

	--msal-proxy-path
		Display the path to the MSAL proxy application.

	-n
		Do not print a newline.
	`

	if os.Getenv("GO_WANT_HELPER_PROCESS") == "" {
		t.Skip("Skipped because it is not a real test, but rather a mocked executable")
	}

	var argv []string
	begin := slices.Index(os.Args, "--")
	if begin != -1 {
		argv = os.Args[begin+1:]
	}

	// Action
	// We use the last CLI argument to determine the networking mode behavior.
	netmode := argv[len(argv)-1]
	a := strings.TrimSpace(strings.Join(argv[:len(argv)-1], " "))
	if netmode == "error" {
		fmt.Fprintln(os.Stderr, "Access denied")
		os.Exit(2)
	}
	switch a {
	case "wslinfo --networking-mode -n":
		fmt.Fprint(os.Stdout, netmode)
		fmt.Fprintf(os.Stdout, "\nexit status 0\n")
		os.Exit(0)

	case "wslinfo --networking-mode":
		fmt.Fprintln(os.Stdout, netmode)
		fmt.Fprintf(os.Stdout, "\nexit status 0\n")
		os.Exit(0)

	default:
		fmt.Fprintf(os.Stderr, "Invalid argument: [%s]\n", a)
		fmt.Fprintln(os.Stderr, errorUsage)
		os.Exit(1)
	}
}

var (
	//go:linkname defaultOptions github.com/canonical/ubuntu-pro-for-wsl/windows-agent/internal/daemon.defaultOptions
	defaultOptions struct {
		wslSystemCmd         []string
		wslCmdEnv            []string
		getAdaptersAddresses func(family uint32, flags uint32, reserved uintptr, adapterAddresses *IPAdapterAddresses, sizePointer *uint32) (errcode error)
	}
)

// DefaultNetworkDetectionToMock sets the default options for the daemon package with mocks for success of upper level tests.
func DefaultNetworkDetectionToMock() {
	testdetection.MustBeTesting()

	m := NewHostIPConfigMock(MultipleHyperVAdaptersInList)

	defaultOptions.wslSystemCmd = []string{
		os.Args[0],
		"-test.run",
		"TestWithWslSystemMock",
		"--",
		"wslinfo",
		"--networking-mode",
		"-n",
		"nat",
	}
	defaultOptions.wslCmdEnv = []string{"GO_WANT_HELPER_PROCESS=1"}
	defaultOptions.getAdaptersAddresses = m.GetAdaptersAddresses
}

// TestWithWslSystemMock is a faux test used to mock commands running via `wsl --system`.
func TestWithWslSystemMock(t *testing.T) {
	MockWslSystemCmd(t)
}
