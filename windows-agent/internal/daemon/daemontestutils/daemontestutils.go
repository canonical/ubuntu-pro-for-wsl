// Package daemontestutils exports test helpers to be used in other packages that need to change internal behaviors of the daemon.
package daemontestutils

import (
	"fmt"
	"os"
	"slices"
	"strings"
	"testing"
	//nolint:revive,nolintlint // needed for go:linkname, but only used in tests. nolintlint as false positive then.
	_ "unsafe"

	"github.com/canonical/ubuntu-pro-for-wsl/common/testdetection"
	"github.com/canonical/ubuntu-pro-for-wsl/windows-agent/internal/daemon/netmonitoring"
	"github.com/google/uuid"
	"golang.org/x/exp/maps"
)

// MockWslSystemCmd mocks commands running inside the WSL system distro.
// To use this in higher level package tests, call `DefaultNetworkDetectionToMock()` in the test package `init` function,
// or have a `With...()` function changing the options passed to `daemon.Serve()`.
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
	// We expect at least 4 arguments after the "--" (for example: "mirrored --system wslinfo --networking-mode").
	if begin == -1 || len(os.Args) < begin+4 {
		fmt.Fprintf(os.Stderr, "Invalid arguments: [%v]\n", os.Args)
		fmt.Fprintln(os.Stderr, errorUsage)
		os.Exit(1)
	}
	// We use the first CLI argument (after the "--") to determine the networking mode behavior.
	netmode := os.Args[begin+1]
	argv = os.Args[begin+2:]

	// Action
	a := strings.TrimSpace(strings.Join(argv[:len(argv)-1], " "))
	if netmode == "error" {
		fmt.Fprintln(os.Stderr, "Access denied")
		os.Exit(2)
	}
	switch a {
	case "--system wslinfo --networking-mode -n":
		fmt.Fprint(os.Stdout, netmode)
		fmt.Fprintf(os.Stdout, "\nexit status 0\n")
		os.Exit(0)

	case "--system wslinfo --networking-mode":
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
		wslSystemCmd          []string
		wslCmdEnv             []string
		getAdaptersAddresses  func(family uint32, flags uint32, reserved uintptr, adapterAddresses *IPAdapterAddresses, sizePointer *uint32) (errcode error)
		netMonitoringProvider func() (netmonitoring.DevicesAPI, error)
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
		"nat",
	}
	defaultOptions.wslCmdEnv = []string{"GO_WANT_HELPER_PROCESS=1"}
	defaultOptions.getAdaptersAddresses = m.GetAdaptersAddresses
	defaultOptions.netMonitoringProvider = func() (netmonitoring.DevicesAPI, error) {
		return &NetMonitoringMockAPI{}, nil
	}
}

// NetDevicesMockAPIWithAddedWSL returns a NetAdaptersAPIProvider fuinction with a new WSL adapter added to the future list of adapters.
// The returnAfter channel is used to introduce asynchrony to the test and may be used to send errors to the waiting goroutine.
func NetDevicesMockAPIWithAddedWSL(returnAfter <-chan error) netmonitoring.DevicesAPIProvider {
	return func() (netmonitoring.DevicesAPI, error) {
		before := map[string]string{
			uuid.New().String(): "Wireless LAN adapter Wi-Fi",
			uuid.New().String(): "Ethernet adapter Ethernet",
		}

		after := map[string]string{
			uuid.New().String(): "Ethernet adapter vEthernet (WSL (Hyper-V firewall))",
			uuid.New().String(): "vSwitch (WSL (Hyper-V firewall))",
			"Descriptions":      "yet_another_new",
		}
		maps.Copy(after, before)

		return &NetMonitoringMockAPI{
			Before: before,
			After:  after,
			WaitForDeviceChangesImpl: func() error {
				// Introduces some asynchrony to the test.
				if returnAfter != nil {
					return <-returnAfter
				}
				return nil
			},
		}, nil
	}
}
