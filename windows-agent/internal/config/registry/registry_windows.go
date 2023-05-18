package registry

import (
	"errors"
	"syscall"

	"golang.org/x/sys/windows/registry"
)

// Windows is the Windows registry.
type Windows struct{}

// HKCUCreateKey creates a key in the specified path under the HK_CURRENT_USER registry.
func (Windows) HKCUCreateKey(path string, access uint32) (newk uintptr, err error) {
	key, _, err := registry.CreateKey(registry.CURRENT_USER, path, access)
	if errors.Is(err, syscall.Errno(5)) { // Access is denied
		return 0, ErrAccessDenied
	}
	return uintptr(key), err
}

// HKCUOpenKey opens a key in the specified path under the HK_CURRENT_USER registry.
func (Windows) HKCUOpenKey(path string, access uint32) (uintptr, error) {
	key, err := registry.OpenKey(registry.CURRENT_USER, path, access)
	if errors.Is(err, registry.ErrNotExist) {
		return 0, ErrKeyNotExist
	}
	if errors.Is(err, syscall.Errno(5)) { // Access is denied
		return 0, ErrAccessDenied
	}
	return uintptr(key), err
}

// CloseKey releases a key.
func (Windows) CloseKey(k uintptr) {
	// The error is not actionable, so no point in reporting it
	_ = registry.Key(k).Close()
}

// ReadValue returns the value of the specified field in the specified key.
func (Windows) ReadValue(k uintptr, field string) (value string, err error) {
	value, _, err = registry.Key(k).GetStringValue(field)
	if errors.Is(err, registry.ErrNotExist) {
		return value, ErrFieldNotExist
	}
	return value, err
}

// WriteValue writes the provided value into the specified field of key k.
func (Windows) WriteValue(k uintptr, field string, value string) (err error) {
	return registry.Key(k).SetStringValue(field, value)
}
