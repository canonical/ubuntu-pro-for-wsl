package registry

import (
	"errors"
	"strings"
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
func (Windows) ReadValue(k uintptr, field string) (string, error) {
	var acc error

	// Try to read single-line string
	value, _, err := registry.Key(k).GetStringValue(field)
	if errors.Is(err, registry.ErrNotExist) {
		return value, ErrFieldNotExist
	} else if err != nil {
		acc = errors.Join(acc, err)
	} else {
		return value, nil
	}

	// Try to read multi-line string
	lines, _, err := registry.Key(k).GetStringsValue(field)
	if errors.Is(err, registry.ErrNotExist) {
		return value, ErrFieldNotExist
	} else if err != nil {
		acc = errors.Join(acc, err)
	} else {
		return strings.Join(lines, "\n"), nil
	}

	return "", acc
}

// WriteValue writes the provided value into the specified field of key k.
func (Windows) WriteValue(k uintptr, field string, value string) error {
	var acc error

	if !strings.ContainsRune(value, '\n') {
		// Single line string: we try storing a regular string
		// This can fail if this field is already multi-line
		if err := registry.Key(k).SetStringValue(field, value); err != nil {
			acc = errors.Join(acc, err)
		} else {
			return nil
		}
	}

	// Multi-line string
	if err := registry.Key(k).SetStringsValue(field, strings.Split(value, "\n")); err != nil {
		acc = errors.Join(acc, err)
	} else {
		return nil
	}

	return acc
}
