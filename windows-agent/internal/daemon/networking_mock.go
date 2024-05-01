package daemon

import (
	"errors"
	"net"
	"unsafe"

	"github.com/canonical/ubuntu-pro-for-wsl/common/testdetection"
)

type mockIPAdaptersState int

const (
	mockError mockIPAdaptersState = iota
	emptyList
	noHyperVAdapterInList
	singleHyperVAdapterInList
	multipleHyperVAdaptersInList
)

// newHostIPConfigMock initializes a mockIPConfig object with the state provided so it can be used instead of the real GetAdaptersAddresses Win32 API.
func newHostIPConfigMock(state mockIPAdaptersState) mockIPConfig {
	testdetection.MustBeTesting()

	m := mockIPConfig{state: state}

	adaptersList := []mockIPAddrsTemplate{
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
	case noHyperVAdapterInList:
		m.addrs = adaptersList
	case singleHyperVAdapterInList:
		m.addrs = append(
			adaptersList,
			mockIPAddrsTemplate{
				friendlyName: "Ethernet adapter vEthernet (WSL)",
				desc:         "Hyper-V Virtual Ethernet Adapter",
				ip:           localIP,
			},
		)
	case multipleHyperVAdaptersInList:
		m.addrs = append(
			adaptersList,
			mockIPAddrsTemplate{
				friendlyName: "Ethernet adapter vEthernet (Default Switch)",
				desc:         "Hyper-V Virtual Ethernet Adapter",
				ip:           net.IPv4(172, 27, 48, 1),
			},
			mockIPAddrsTemplate{
				friendlyName: "Ethernet adapter vEthernet (WSL (Hyper-V firewall))",
				desc:         "Hyper-V Virtual Ethernet Adapter #2",
				ip:           localIP,
			},
		)
	}

	return m
}

// GetAdaptersAddresses is a mock implementation of the GetAdaptersAddresses Win32 API, based on the state of the mockIPConfig object.
func (m *mockIPConfig) GetAdaptersAddresses(family uint32, flags uint32, reserved uintptr, adapterAddresses *ipAdapterAddresses, sizePointer *uint32) (errcode error) {
	testdetection.MustBeTesting()

	switch m.state {
	case mockError:
		return errors.New("mock error")
	case emptyList:
		return nil
	default:
		return fillBufferFromTemplate(adapterAddresses, sizePointer, m.addrs)
	}
}

// fillBufferFromTemplate fills a pre-allocated buffer of ipAdapterAddresses with the data from the mockIPAddrsTemplate.
func fillBufferFromTemplate(adaptersAddresses *ipAdapterAddresses, sizePointer *uint32, mockIPAddrsTemplate []mockIPAddrsTemplate) error {
	count := uint32(len(mockIPAddrsTemplate))
	objSize := uint32(unsafe.Sizeof(ipAdapterAddresses{}))
	bufSizeNeeded := count * objSize
	if *sizePointer < bufSizeNeeded {
		return ERROR_BUFFER_OVERFLOW
	}

	//nolint:gosec // Using unsafe to manipulate pointers mimicking the Win32 API, only used in tests.
	begin := unsafe.Pointer(adaptersAddresses)
	for _, addr := range mockIPAddrsTemplate {
		next := unsafe.Add(begin, int(objSize)) // next = ++begin

		ptr := (*ipAdapterAddresses)(begin)
		ptr.fillFromTemplate(&addr, (*ipAdapterAddresses)(next))

		begin = next
	}
	*sizePointer = bufSizeNeeded
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

// mockIPConfig holds the state to control the mock implementation of the GetAdaptersAddresses Win32 API.
type mockIPConfig struct {
	state mockIPAdaptersState
	addrs []mockIPAddrsTemplate
}

// mockIPAddrsTemplate is a template to fill the ipAdapterAddresses struct with mock data.
type mockIPAddrsTemplate struct {
	friendlyName, desc string
	ip                 net.IP
}
