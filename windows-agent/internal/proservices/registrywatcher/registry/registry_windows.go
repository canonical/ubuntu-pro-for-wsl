package registry

import (
	"errors"
	"fmt"
	"strings"
	"syscall"

	"github.com/ubuntu/decorate"
	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
)

// Windows is the Windows registry.
type Windows struct{}

// HKCUOpenKey opens a key in the specified path under the HK_CURRENT_USER registry with read permissions.
func (Windows) HKCUOpenKey(path string) (Key, error) {
	key, err := registry.OpenKey(registry.CURRENT_USER, path, registry.READ)
	if errors.Is(err, registry.ErrNotExist) {
		return 0, ErrKeyNotExist
	}
	if errors.Is(err, syscall.Errno(5)) { // Access is denied
		return 0, ErrAccessDenied
	}
	return Key(key), err
}

// HKCUCreateKey creaters a key in the specified path under the HK_CURRENT_USER registry with write permissions.
func (Windows) HKCUCreateKey(path string) (Key, error) {
	key, _, err := registry.CreateKey(registry.CURRENT_USER, path, registry.READ|registry.WRITE)
	if errors.Is(err, registry.ErrNotExist) {
		return 0, ErrKeyNotExist
	}
	if errors.Is(err, syscall.Errno(5)) { // Access is denied
		return 0, ErrAccessDenied
	}
	return Key(key), err
}

// CloseKey releases a key.
func (Windows) CloseKey(k Key) {
	// The error is not actionable, so no point in reporting it
	_ = registry.Key(k).Close()
}

// ReadValue returns the value of the specified field in the specified key.
func (Windows) ReadValue(k Key, field string) (string, error) {
	var errs error

	// Try to read single-line string
	value, _, err := registry.Key(k).GetStringValue(field)
	if errors.Is(err, registry.ErrNotExist) {
		return value, ErrFieldNotExist
	} else if err != nil {
		errs = errors.Join(errs, err)
	} else {
		return value, nil
	}

	// Try to read multi-line string
	lines, _, err := registry.Key(k).GetStringsValue(field)
	if errors.Is(err, registry.ErrNotExist) {
		return value, ErrFieldNotExist
	} else if err != nil {
		errs = errors.Join(errs, err)
	} else {
		return strings.Join(lines, "\n"), nil
	}

	return "", errs
}

// WriteValue writes the value to the specified field in the specified key.
func (Windows) WriteValue(k Key, field, value string, multiLine bool) error {
	var err error
	if multiLine {
		err = registry.Key(k).SetStringsValue(field, strings.Split(value, "\n"))
	} else {
		err = registry.Key(k).SetStringValue(field, value)
	}

	if errors.Is(err, registry.ErrNotExist) {
		return ErrKeyNotExist
	}
	if errors.Is(err, syscall.Errno(5)) {
		return ErrAccessDenied
	}
	return err
}

// ReadIntegerValue reads the value of the specified integer (DWORD or QWORD) field in the specified key.
func (Windows) ReadIntegerValue(k Key, field string) (uint64, error) {
	value, _, err := registry.Key(k).GetIntegerValue(field)
	if errors.Is(err, registry.ErrNotExist) {
		return 0, ErrFieldNotExist
	} else if err != nil {
		return 0, err
	}

	return value, nil
}

// SetDWordValue sets the value of the specified DWORD field in the specified key.
func (Windows) SetDWordValue(k Key, field string, value uint32) error {
	err := registry.Key(k).SetDWordValue(field, value)
	if errors.Is(err, registry.ErrNotExist) {
		return ErrKeyNotExist
	}
	if errors.Is(err, syscall.Errno(5)) {
		return ErrAccessDenied
	}
	return err
}

// RegNotifyChangeKeyValue creates an event and attaches it to a registry key.
// Modifying that key or its children will trigger the event.
// This trigger can be detected by WaitForSingleObject.
func (Windows) RegNotifyChangeKeyValue(k Key) (ev Event, err error) {
	defer decorate.OnError(&err, "could not start watching registry")

	event, err := windows.CreateEvent(nil, 1, 0, nil)
	if err != nil {
		return 0, fmt.Errorf("could not create event: %v", err)
	}

	// notifyFilter indicates the changes that should be reported.
	var notifyFilter uint32

	// Notify the caller if a subkey is added or deleted.
	notifyFilter |= windows.REG_NOTIFY_CHANGE_NAME

	// Notify the caller of changes to a value of the key.
	// This can include adding or deleting a value, or changing an existing value.
	notifyFilter |= windows.REG_NOTIFY_CHANGE_LAST_SET

	// Ensure that the Go scheduler does not mess with the wait.
	notifyFilter |= windows.REG_NOTIFY_THREAD_AGNOSTIC

	err = windows.RegNotifyChangeKeyValue(windows.Handle(k), true, notifyFilter, event, true)
	if err != nil {
		return 0, fmt.Errorf("in call to RegNotifyChangeKeyValue: %v", err)
	}

	return Event(event), nil
}

// WaitForSingleObject waits until the event is triggered. This is a blocking function.
func (Windows) WaitForSingleObject(ev Event) (err error) {
	if _, err := windows.WaitForSingleObject(windows.Handle(ev), windows.INFINITE); err != nil {
		return fmt.Errorf("in call to WaitForSingleObject: %v", err)
	}

	return nil
}

// SetEvent triggers an event.
func (Windows) SetEvent(ev Event) (err error) {
	if err := windows.SetEvent(windows.Handle(ev)); err != nil {
		return fmt.Errorf("in call to SetEvent: %v", err)
	}

	return nil
}

// CloseEvent releases the event.
func (Windows) CloseEvent(ev Event) {
	_ = windows.CloseHandle(windows.Handle(ev))
}
