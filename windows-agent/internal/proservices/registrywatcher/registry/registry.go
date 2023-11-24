// Package registry simplifies read/write access to the registry and allows for
// mocking during tests.
package registry

import "errors"

// Although the stdlibs' registry and syscall packages return typed errors, these are
// only defined in their _windows files. We convert the relevant ones here to
// cross-platform errors.
var (
	// ErrKeyNotExist is returned when attempting to open a key that does not exist.
	ErrKeyNotExist = errors.New("the key does not exist")

	// ErrFieldNotExist is returned when attempting to read a key field that does not exist.
	ErrFieldNotExist = errors.New("the field does not exist")

	// ErrAccessDenied is printed when an action is blocked by the key's security descriptor.
	//
	// It is NOT used when the action is blocked by access rights such READ or WRITE.
	// In that case we allow the syscall error to bubble up, as we don't need to catch it.
	ErrAccessDenied = errors.New("access denied")

	// ErrMock is the error returned when everything went fine but the mock
	// setup requested an error to be thrown.
	ErrMock = errors.New("error triggered by mock setup")
)

// Event is a void pointer to a Windows event.
type Event uintptr

// Key is a void pointer to a Windows registry key.
type Key uintptr
