// Package netmonitoring defines the network devices monitoring API separated from the daemon package
// to avoid cyclic dependencies on tests, as the sibling package daemontestutils needs some this interface declaration.
package netmonitoring

// DevicesAPI is an interface for interacting with the network devices on the host.
type DevicesAPI interface {
	// Close releases the resources associated with this object and cancels any outstanding wait operation.
	Close()

	// ListDevices returns the GUIDs of the network adapters on the host.
	ListDevices() ([]string, error)

	// GetDeviceConnectionName returns the connection name of the network adapter with the given GUID.
	GetDeviceConnectionName(guid string) (string, error)

	// WaitForChanges blocks the caller until the system triggers a notification of changes to the network adapters.
	// It returns nil if the notification is triggered or an error if the context is cancelled or an error occurs.
	// The wait is cancellable by calling Close().
	WaitForDeviceChanges() error
}

// DevicesAPIProvider is a function that returns a new instance of NetAdaptersAPI or an error.
type DevicesAPIProvider func() (DevicesAPI, error)
