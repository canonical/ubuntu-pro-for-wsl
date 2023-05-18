package registry

// Windows is the Windows registry. Any interaction with it will panic.
type Windows struct{}

// HKCUCreateKey creates a key in the specified path under the HK_CURRENT_USER registry.
func (Windows) HKCUCreateKey(path string, access uint32) (newk uintptr, err error) {
	panic("the Windows registry is not available on Linux")
}

// HKCUOpenKey opens a key in the specified path under the HK_CURRENT_USER registry.
func (Windows) HKCUOpenKey(path string, access uint32) (key uintptr, err error) {
	panic("the Windows registry is not available on Linux")
}

// CloseKey releases a key.
func (Windows) CloseKey(k uintptr) {
	panic("the Windows registry is not available on Linux")
}

// ReadValue returns the value of the specified field in the specified key.
func (Windows) ReadValue(k uintptr, field string) (value string, err error) {
	panic("the Windows registry is not available on Linux")
}

// WriteValue writes the provided value into the specified field of key k.
func (Windows) WriteValue(k uintptr, field string, value string) (err error) {
	panic("the Windows registry is not available on Linux")
}
