package registry

import (
	"errors"
	"sync/atomic"
)

// Mock is a fake registry stored in memory.
// It can only have one key:
//
//	HKCU\Software\Canonical\UbuntuPro
type Mock struct {
	// UbuntuProData contains the values stored in the UbuntuPro key.
	UbuntuProData map[string]string

	// KeyExists indicates whether the UbuntuPro key already exists in the registry.
	KeyExists bool

	// KeyExists indicates whether the current user has write access to the UbuntuPro key
	KeyIsReadOnly bool

	// Error triggers so the tests can mock system errors
	Errors uint32

	// OpenKeyCount can be used by tests to ensure they don't leak by asserting this is 0 at the end of the test
	OpenKeyCount *atomic.Int32
}

// Error triggers so the tests can mock system errors.
const (
	MockErrOnCreateKey uint32 = 1 << iota
	MockErrOnOpenKey
	MockErrReadValue
	MockErrOnWriteValue
)

// NewMock initializes a mocked registry.
func NewMock() *Mock {
	return &Mock{
		UbuntuProData: make(map[string]string),
		OpenKeyCount:  &atomic.Int32{},
	}
}

// HKCUCreateKey mocks creating a key in the specified path under the HK_CURRENT_USER registry.
func (r *Mock) HKCUCreateKey(path string, access uint32) (newk uintptr, err error) {
	if path != `Software\Canonical\UbuntuPro` {
		panic(`Attempted to access registry outside of HKCU\Software\Canonical\UbuntuPro`)
	}

	if r.Errors&MockErrOnCreateKey != 0 {
		return newk, ErrMock
	}

	if r.KeyExists && r.KeyIsReadOnly && isWrite(access) {
		return 0, ErrAccessDenied
	}

	r.KeyExists = true
	r.OpenKeyCount.Add(1)

	// Since we always deal with the same key, we return its access permissions
	// as that is all the info we need from it.
	return uintptr(access), nil
}

// HKCUOpenKey mocks opening a key in the specified path under the HK_CURRENT_USER registry.
func (r *Mock) HKCUOpenKey(path string, access uint32) (uintptr, error) {
	if path != `Software\Canonical\UbuntuPro` {
		panic(`Attempted to access registry outside of HKCU\Software\Canonical\UbuntuPro`)
	}

	if r.Errors&MockErrOnOpenKey != 0 {
		return 0, ErrMock
	}

	if !r.KeyExists {
		return 0, ErrKeyNotExist
	}

	if r.KeyIsReadOnly && isWrite(access) {
		return 0, ErrAccessDenied
	}

	r.OpenKeyCount.Add(1)

	// Since we always deal with the same key, we return its access permissions
	// as that is all the info we need from it.
	return uintptr(access), nil
}

// CloseKey mocks releasing a key.
func (r *Mock) CloseKey(k uintptr) {
	r.OpenKeyCount.Add(-1)
}

// ReadValue returns the value of the specified field in the specified key.
func (r Mock) ReadValue(k uintptr, field string) (value string, err error) {
	if k == 0 {
		return value, errors.New("Null key")
	}

	if r.Errors&MockErrReadValue != 0 {
		return value, ErrMock
	}

	if !isRead(uint32(k)) {
		return value, errors.New("key was not opened with READ access")
	}
	v, ok := r.UbuntuProData[field]
	if !ok {
		return v, ErrFieldNotExist
	}
	return v, nil
}

// WriteValue writes the provided value into the specified field of key k.
func (r *Mock) WriteValue(k uintptr, field string, value string) (err error) {
	if k == 0 {
		return errors.New("Null key")
	}

	if r.Errors&MockErrOnWriteValue != 0 {
		return ErrMock
	}

	if !isWrite(uint32(k)) {
		return errors.New("key was not opened with WRITE access")
	}

	r.UbuntuProData[field] = value
	return nil
}

func isRead(access uint32) bool {
	// Bit mask of the lower 16 bits
	return (access & READ & 0xffff) != 0
}

func isWrite(access uint32) bool {
	// Bit mask of the lower 16 bits
	return (access & WRITE & 0xffff) != 0
}
