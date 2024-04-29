package daemon

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"slices"
	"strings"
	"syscall"
	"testing"
)

type mockIPAdaptersState int

const (
	mockError mockIPAdaptersState = iota
	emptyList
	noHyperVAdapterInList
	singleHyperVAdapterInList
	ok
)

func newHostIPConfigMock(state mockIPAdaptersState) mockIPConfig {
	return mockIPConfig{state: state}
}

func (m *mockIPConfig) getAdaptersAddresses() (head ipAdapterAddresses, err error) {
	adaptersList := []mockIPAdapterAddresses{
		{ipconfig: m, name: "Ethernet adapter Ethernet", desc: " Realtek(R) PCI(e) Ethernet Controller", ip: net.IPv4(192, 168, 17, 15)},
		{ipconfig: m, name: "Wireless LAN adapter Wi-Fi", desc: "Qualcomm Atheros QCA9377 Wireless Network Adapter", ip: net.IPv4(192, 168, 17, 4)},
		{ipconfig: m, name: "Wireless LAN adapter Local Area Connection* 1", desc: " Microsoft Wi-Fi Direct Virtual Adapter", ip: nil},
	}

	// prefer not to listen on public interfaces if possible.
	localIP := getLocalPrivateIP()
	if localIP == nil {
		localIP = net.IPv4(0, 0, 0, 0)
	}

	switch m.state {
	case mockError:
		return nil, errors.New("mock error")
	case emptyList:
		return nil, nil
	case noHyperVAdapterInList:
		m.adapters = adaptersList
	case singleHyperVAdapterInList:
		m.adapters = append(adaptersList, mockIPAdapterAddresses{ipconfig: m, name: "Ethernet adapter vEthernet (WSL (Hyper-V firewall))", desc: " Hyper-V Virtual Ethernet Adapter", ip: localIP})
	case ok:
		m.adapters = append(adaptersList,
			mockIPAdapterAddresses{ipconfig: m, name: "Ethernet adapter vEthernet (Default Switch)", desc: " Hyper-V Virtual Ethernet Adapter", ip: net.IPv4(172, 27, 48, 1)},
			mockIPAdapterAddresses{ipconfig: m, name: "Ethernet adapter vEthernet (WSL (Hyper-V firewall))", desc: " Hyper-V Virtual Ethernet Adapter #2", ip: localIP})
	}

	return &m.adapters[0], nil
}

// getLocalPrivateIP returns one non loopback local private IP of the host.
func getLocalPrivateIP() net.IP {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return nil
	}
	for _, addr := range addrs {
		if ipnet, ok := addr.(*net.IPNet); ok && !ipnet.IP.IsLoopback() && ipnet.IP.IsPrivate() {
			return ipnet.IP
		}
	}
	return nil
}
func (m *mockIPAdapterAddresses) Next() ipAdapterAddresses {
	return m.ipconfig.next()
}

func (m *mockIPAdapterAddresses) Description() string {
	return m.desc
}

func (m *mockIPAdapterAddresses) IP() net.IP {
	return m.ip
}

func (m *mockIPAdapterAddresses) FriendlyName() string {
	return m.name
}

func (m *mockIPConfig) next() ipAdapterAddresses {
	if m.current+1 >= len(m.adapters) {
		return nil
	}
	m.current++
	return &m.adapters[m.current]
}

type mockIPConfig struct {
	state    mockIPAdaptersState
	adapters []mockIPAdapterAddresses
	current  int
}
type mockIPAdapterAddresses struct {
	ipconfig   *mockIPConfig
	name, desc string
	ip         net.IP
}

type mockWslSystem struct {
	netmode  string
	extraEnv []string
	cmdError bool
}

func newWslSystemMock(netmode string, extraEnv []string, cmdError bool) *mockWslSystem {
	return &mockWslSystem{netmode: netmode, extraEnv: extraEnv, cmdError: cmdError}
}

func (m *mockWslSystem) Command(ctx context.Context, name string, args ...string) *exec.Cmd {
	if !testing.Testing() {
		panic("mockWslSystem can only be used within a test")
	}

	goArgs := append([]string{"test", "-run", "^TestWithWslSystemMock$", "--", name}, args...)
	// Switches
	env := append(os.Environ(), m.extraEnv...)
	env = append(env,
		fmt.Sprintf("%s=1", "UP4W_MOCK_EXECUTABLE"),
		fmt.Sprintf("%s=%s", "UP4W_MOCK_NETWORKING_MODE", m.netmode),
	)
	if m.cmdError {
		env = append(env, fmt.Sprintf("%s=1", "UP4W_MOCK_NETWORKING_MODE_ERROR"))
	}

	//nolint: gosec // Subprocess launched with variable (gosec) intentionally so we can mock it.
	c := exec.CommandContext(ctx, "go", goArgs...)
	c.Env = env
	return c
}

// WslSystemMock mocks commands running inside the WSL system distro.
// Add it to your package_test with:
//
//	func TestWithWslSystemMock(t *testing.T) { daemon.WslSystemMock(t) }
//
//nolint:thelper // This is a faux test used to mock commands running via `wsl -- system`
func WslSystemMock(t *testing.T) {
	// Setup
	if t.Name() != "TestWithWslSystemMock" {
		panic("The WslSystemMock faux test must be named TestWithWslSystemMock")
	}

	const errorUsage = `
wslinfo usage:
	--networking-mode
		Display current networking mode.

	--msal-proxy-path
		Display the path to the MSAL proxy application.

	-n
		Do not print a newline.
	`

	if os.Getenv("UP4W_MOCK_EXECUTABLE") == "" {
		t.Skip("Skipped because it is not a real test, but rather a mocked executable")
	}

	var argv []string
	begin := slices.Index(os.Args, "--")
	if begin != -1 {
		argv = os.Args[begin+1:]
	}

	// Action
	exit := func(args []string) int {
		a := strings.TrimSpace(strings.Join(args, " "))
		netmode := os.Getenv("UP4W_MOCK_NETWORKING_MODE")
		if netmode == "" {
			netmode = "nat"
		}
		if os.Getenv("UP4W_MOCK_NETWORKING_MODE_ERROR") != "" {
			fmt.Fprintln(os.Stderr, "Access denied")
			return 2
		}
		switch a {
		case "wslinfo --networking-mode -n":
			fmt.Fprint(os.Stdout, netmode)
			return 0

		case "wslinfo --networking-mode":
			fmt.Fprintln(os.Stdout, netmode)
			return 0

		default:
			fmt.Fprintf(os.Stderr, "Invalid argument: [%s]\n", a)
			fmt.Fprintln(os.Stderr, errorUsage)
			return 1
		}
	}(argv)

	// Ensure we clean-exit.

	if exit == 0 {
		// testing library only prints this line when it fails
		// Manually printing it means that we can simply remove the last two lines to get the true output
		fmt.Fprintf(os.Stdout, "\nexit status 0\n")
	}
	syscall.Exit(exit)
}
