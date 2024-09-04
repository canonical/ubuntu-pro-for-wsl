package netmonitoring

import (
	"fmt"
	"path/filepath"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
)

// DevicesAPIWindows is an implementation of the DevicesAPI interface relying on the well-known registry path `HKLM:SYSTEM\CurrentControlSet\Control\Network\{4D36E972-E325-11CE-BFC1-08002BE10318}` provided by the OS.
type DevicesAPIWindows struct {
	k registry.Key
}

// Close releases the resources associated with this object.
func (a DevicesAPIWindows) Close() {
	_ = a.k.Close()
}

// DefaultAPIProvider returns a new instance of DevicesAPIWindows or an error if it fails to open the registry key.
func DefaultAPIProvider() (DevicesAPI, error) {
	k, err := registry.OpenKey(windows.HKEY_LOCAL_MACHINE, `SYSTEM\CurrentControlSet\Control\Network\{4D36E972-E325-11CE-BFC1-08002BE10318}`, registry.READ)
	return DevicesAPIWindows{k: k}, err
}

// ListDevices returns the GUIDs of the network adapters on the host.
func (a DevicesAPIWindows) ListDevices() ([]string, error) {
	return a.k.ReadSubKeyNames(-1) // This could potentially be implemented in terms of `GetInterfacesInfo`.
}

// GetDeviceConnectionName returns the connection name of the network adapter with the given GUID.
func (a DevicesAPIWindows) GetDeviceConnectionName(guid string) (string, error) {
	// This could be implemented in terms of GetAdaptersAddresses. All other APIs considered would depend on the registry anyway.
	sk, err := registry.OpenKey(a.k, filepath.Join(guid, "Connection"), registry.READ)
	if err != nil {
		return "", fmt.Errorf("could not read the connection info from adapter GUID %s: %v", guid, err)
	}
	defer sk.Close()

	// Ignoring the registry value type trusting the OS will never create non-string values for this key.
	v, _, err := sk.GetStringValue("Name")
	if err != nil {
		return "", fmt.Errorf("could not read the connection name from adapter GUID %s: %v", guid, err)
	}
	return v, nil
}

// WaitForDeviceChanges blocks the caller until the system triggers a notification of changes to the network adapters.
// The wait can be cancelled by calling Close().
func (a DevicesAPIWindows) WaitForDeviceChanges() error {
	// This part could be implemented in terms of CM_Register_Notification, if we find a way to set a Win32 callback without relying on CGo.
	// Wait synchronuosly on notifications if a subkey is added or deleted, or changes to a value of the key, including adding or deleting a value, or changing an existing value.
	return windows.RegNotifyChangeKeyValue(windows.Handle(a.k), true, windows.REG_NOTIFY_CHANGE_NAME|windows.REG_NOTIFY_CHANGE_LAST_SET, windows.Handle(0), false)
}
