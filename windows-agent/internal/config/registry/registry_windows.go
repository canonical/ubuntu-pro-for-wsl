package registry

import (
	"errors"
	"strings"
	"syscall"

	"golang.org/x/sys/windows/registry"
)

// Windows is the Windows registry.
type Windows struct{}

// HKCUOpenKey opens a key in the specified path under the HK_CURRENT_USER registry.
func (Windows) HKCUOpenKey(path string) (uintptr, error) {
	key, err := registry.OpenKey(registry.CURRENT_USER, path, registry.READ)
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
