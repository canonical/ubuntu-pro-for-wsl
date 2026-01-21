package registry

// Windows is the Windows registry. Any interaction with it will panic.
type Windows struct{}

// HKCUOpenKey opens a key in the specified path under the HK_CURRENT_USER registry with read permissions.
func (Windows) HKCUOpenKey(path string) (Key, error) {
	panic("the Windows registry is not available on Linux")
}

// HKCUCreateKey creates a key in the specified path under the HK_CURRENT_USER registry with write permissions.
func (Windows) HKCUCreateKey(path string) (Key, error) {
	panic("the Windows registry is not available on Linux")
}

// CloseKey releases a key.
func (Windows) CloseKey(k Key) {
	panic("the Windows registry is not available on Linux")
}

// ReadValue returns the value of the specified field in the specified key.
func (Windows) ReadValue(k Key, field string) (string, error) {
	panic("the Windows registry is not available on Linux")
}

// WriteValue writes the value to the specified field in the specified key.
func (Windows) WriteValue(k Key, field, value string, multiLine bool) error {
	panic("the Windows registry is not available on Linux")
}

// ReadDWordValue reads the value of the specified DWORD integer field in the specified key.
func (Windows) ReadDWordValue(k Key, field string) (uint64, error) {
	panic("the Windows registry is not available on Linux")
}

// SetDWordValue sets the value of the specified DWORD field in the specified key.
func (Windows) SetDWordValue(k Key, field string, value uint32) error {
	panic("the Windows registry is not available on Linux")
}

// RegNotifyChangeKeyValue creates an event and attaches it to a registry key.
// Modifying that key or its children will trigger the event.
// This trigger can be detected by WaitForSingleObject.
func (Windows) RegNotifyChangeKeyValue(k Key) (ev Event, err error) {
	panic("the Windows registry is not available on Linux")
}

// WaitForSingleObject waits until the event is triggered. This is a blocking function.
func (Windows) WaitForSingleObject(ev Event) (err error) {
	panic("the Windows registry is not available on Linux")
}

// CloseEvent releases the event.
func (Windows) CloseEvent(ev Event) {
	panic("the Windows registry is not available on Linux")
}
