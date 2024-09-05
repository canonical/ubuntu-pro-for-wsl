package daemontestutils

import (
	"errors"
	"fmt"
	"sync/atomic"

	"golang.org/x/exp/maps" // When migrate to Go 1.23 use "maps" instead.
)

// NetMonitoringMockAPI implements the NetworkAdapterRepository interface for testing purposes.
type NetMonitoringMockAPI struct {
	Before, After map[string]string
	// m.ListDevices() will always fail.
	ErrorOnListDevices error
	// m.ListDevices() will fail only after the first call.
	ErrorOnListDevicesAfterFirstCall error

	GetDeviceConnectionNameError error
	WaitForDeviceChangesImpl     func() error

	listDevicesCalledFirstTime atomic.Bool
}

// Close releases the resources associated with this object and cancels any outstanding wait operation.
func (m *NetMonitoringMockAPI) Close() {}

// ListDevices returns the GUIDs of the network adapters on the host.
func (m *NetMonitoringMockAPI) ListDevices() ([]string, error) {
	if m.ErrorOnListDevices != nil {
		return nil, m.ErrorOnListDevices
	}
	if !m.listDevicesCalledFirstTime.Load() {
		m.listDevicesCalledFirstTime.Store(true)
		return maps.Keys(m.Before), nil
	}
	// After the first call only.
	if m.ErrorOnListDevicesAfterFirstCall != nil {
		return nil, m.ErrorOnListDevicesAfterFirstCall
	}
	return maps.Keys(m.After), nil
}

// GetDeviceConnectionName returns the connection name of the network adapter with the given GUID.
func (m *NetMonitoringMockAPI) GetDeviceConnectionName(guid string) (string, error) {
	if m.GetDeviceConnectionNameError != nil {
		return "", m.GetDeviceConnectionNameError
	}

	if m.Before == nil || m.After == nil {
		return "", errors.New("not implemented")
	}
	if !m.listDevicesCalledFirstTime.Load() {
		if name, ok := m.Before[guid]; ok {
			return name, nil
		}
		return "", fmt.Errorf("device %s not found", guid)
	}

	if name, ok := m.After[guid]; ok {
		return name, nil
	}
	return "", fmt.Errorf("device %s not found", guid)
}

// WaitForDeviceChanges blocks the caller until the system triggers a notification of changes to the network adapters.
func (m *NetMonitoringMockAPI) WaitForDeviceChanges() error {
	if m.WaitForDeviceChangesImpl != nil {
		return m.WaitForDeviceChangesImpl()
	}

	return errors.New("not implemented")
}
