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

// HKCUOpenKey mocks opening a key in the specified path under the HK_CURRENT_USER registry.
func (r *Mock) HKCUOpenKey(path string) (uintptr, error) {
	if path != `Software\Canonical\UbuntuPro` {
		panic(`Attempted to access mock registry outside of HKCU\Software\Canonical\UbuntuPro`)
	}

	if r.Errors&MockErrOnOpenKey != 0 {
		return 0, ErrMock
	}

	if !r.KeyExists {
		return 0, ErrKeyNotExist
	}

	r.OpenKeyCount.Add(1)

	// In the real implementation this integer is a pointer. Here it's just a dummy value.
	return 1, nil
}

// CloseKey mocks releasing a key.
func (r *Mock) CloseKey(k uintptr) {
	r.OpenKeyCount.Add(-1)
}

// ReadValue returns the value of the specified field in the specified key.
func (r Mock) ReadValue(k uintptr, field string) (value string, err error) {
	if k == 0 {
		return value, errors.New("null key")
	}

	if r.Errors&MockErrReadValue != 0 {
		return value, ErrMock
	}

	v, ok := r.UbuntuProData[field]
	if !ok {
		return v, ErrFieldNotExist
	}
	return v, nil
}
