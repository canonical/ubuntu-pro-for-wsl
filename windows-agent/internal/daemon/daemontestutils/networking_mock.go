package daemontestutils

import (
	"errors"
	"math"
	"net"
	"unsafe"

	"github.com/canonical/ubuntu-pro-for-wsl/common/testdetection"
)

// MockIPAdaptersState is an enumeration of the possible states of the MockIPConfig object which influences the result of the mocked GetAdaptersAddresses implementation.
type MockIPAdaptersState int

const (
	// MockError is a state that causes the GetAdaptersAddresses to always return an error.
	MockError MockIPAdaptersState = iota

	// RequiresTooMuchMem is a state that causes the GetAdaptersAddresses to request allocation of MaxUint32 (over the capacity of the real Win32 API).
	RequiresTooMuchMem

	// EmptyList is a state that causes the GetAdaptersAddresses to return an empty list of adapters.
	EmptyList

	// NoHyperVAdapterInList is a state that causes the GetAdaptersAddresses to return a list without any Hyper-V adapter.
	NoHyperVAdapterInList

	// SingleHyperVAdapterInList is a state that causes the GetAdaptersAddresses to return a list with a single Hyper-V adapter, which is the WSL one.
	SingleHyperVAdapterInList

	// MultipleHyperVAdaptersInList is a state that causes the GetAdaptersAddresses to return a list with multiple Hyper-V adapters, one of which is the WSL one.
	MultipleHyperVAdaptersInList
)

// NewHostIPConfigMock initializes a mockIPConfig object with the state provided so it can be used instead of the real GetAdaptersAddresses Win32 API.
func NewHostIPConfigMock(state MockIPAdaptersState) MockIPConfig {
	testdetection.MustBeTesting()

	m := MockIPConfig{state: state}

	adaptersList := []MockIPAddrsTemplate{
		{
			friendlyName: "Ethernet adapter Ethernet",
			desc:         "Realtek(R) PCI(e) Ethernet Controller",
			ip:           net.IPv4(192, 168, 17, 15),
		},
		{
			friendlyName: "Wireless LAN adapter Wi-Fi",
			desc:         "Qualcomm Atheros QCA9377 Wireless Network Adapter",
			ip:           net.IPv4(192, 168, 17, 4),
		},
		{
			friendlyName: "Wireless LAN adapter Local Area Connection* 1",
			desc:         "Microsoft Wi-Fi Direct Virtual Adapter",
			ip:           nil,
		},
	}

	// prefer not to listen on public interfaces if possible.
	localIP := getLocalPrivateIP()
	if localIP == nil {
		localIP = net.IPv4(0, 0, 0, 0)
	}

	switch m.state {
	case NoHyperVAdapterInList:
		m.addrs = adaptersList
	case SingleHyperVAdapterInList:
		m.addrs = append(
			adaptersList,
			MockIPAddrsTemplate{
				friendlyName: "Ethernet adapter vEthernet (WSL)",
				desc:         "Hyper-V Virtual Ethernet Adapter",
				ip:           localIP,
			},
		)
	case MultipleHyperVAdaptersInList:
		m.addrs = append(
			adaptersList,
			MockIPAddrsTemplate{
				friendlyName: "Ethernet adapter vEthernet (Default Switch)",
				desc:         "Hyper-V Virtual Ethernet Adapter",
				ip:           net.IPv4(172, 27, 48, 1),
			},
			MockIPAddrsTemplate{
				friendlyName: "Ethernet adapter vEthernet (WSL (Hyper-V firewall))",
				desc:         "Hyper-V Virtual Ethernet Adapter #2",
				ip:           localIP,
			},
		)
	}

	return m
}

// GetAdaptersAddresses is a mock implementation of the GetAdaptersAddresses Win32 API, based on the state of the mockIPConfig object.
func (m *MockIPConfig) GetAdaptersAddresses(_, _ uint32, _ uintptr, adapterAddresses *IPAdapterAddresses, sizePointer *uint32) (errcode error) {
	testdetection.MustBeTesting()

	switch m.state {
	case MockError:
		return errors.New("mock error")
	case RequiresTooMuchMem:
		*sizePointer = math.MaxUint32
		return ERROR_BUFFER_OVERFLOW
	case EmptyList:
		return nil
	default:
		return fillBufferFromTemplate(adapterAddresses, sizePointer, m.addrs)
	}
}

// fillBufferFromTemplate fills a pre-allocated buffer of ipAdapterAddresses with the data from the mockIPAddrsTemplate.
func fillBufferFromTemplate(adaptersAddresses *IPAdapterAddresses, sizePointer *uint32, mockIPAddrsTemplate []MockIPAddrsTemplate) error {
	count := len(mockIPAddrsTemplate)
	objSize := int(unsafe.Sizeof(IPAdapterAddresses{}))
	bufSizeNeeded := count * objSize
	if bufSizeNeeded >= math.MaxUint32 || bufSizeNeeded < 0 {
		return errors.New("buffer size limit reached")
	}
	//nolint:gosec // Value guaranteed to fit inside uint32.
	bufSz := uint32(bufSizeNeeded)
	if *sizePointer < bufSz {
		return ERROR_BUFFER_OVERFLOW
	}

	//nolint:gosec // Using unsafe to manipulate pointers mimicking the Win32 API, only used in tests.
	begin := unsafe.Pointer(adaptersAddresses)
	for _, addr := range mockIPAddrsTemplate {
		next := unsafe.Add(begin, objSize) // next = ++begin

		ptr := (*IPAdapterAddresses)(begin)
		fillFromTemplate(&addr, ptr, (*IPAdapterAddresses)(next))

		begin = next
	}
	*sizePointer = bufSz
	return nil
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

// MockIPConfig holds the state to control the mock implementation of the GetAdaptersAddresses Win32 API.
type MockIPConfig struct {
	state MockIPAdaptersState
	addrs []MockIPAddrsTemplate
}

// MockIPAddrsTemplate is a template to fill the ipAdapterAddresses struct with mock data.
type MockIPAddrsTemplate struct {
	friendlyName, desc string
	ip                 net.IP
}
