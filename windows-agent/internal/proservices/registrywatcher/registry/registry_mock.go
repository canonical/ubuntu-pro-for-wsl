package registry

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/require"
)

// Mock is a fake registry stored in memory.
type Mock struct {
	// registry contains the registry key database.
	ubuntuPro key
	ubuntu    key
	keyExists bool

	// keyHandles contains the handles to the keys. The Win32API returns void pointers to the
	// key handles, and we mimic this behaviour so we can fit the interface. The user of this
	// library will have a "pointer", which is just a key into this map.
	keyHandles mockedHeap[Key, *keyHandle]

	// eventsHandles contains the eventsHandles. The Win32API returns void pointers to the eventsHandles, and we
	// mimic this behaviour so we can fit the interface. The user of this  library will have
	// a "pointer", which is just a key into this map.
	eventHandles mockedHeap[Event, *eventHandle]

	// Settings to break the registry
	CannotCreate atomic.Bool
	CannotOpen   atomic.Bool
	CannotRead   atomic.Bool
	CannotWatch  atomic.Bool
	CannotWait   atomic.Bool
}

// key mocks a registry key.
type key struct {
	mu      *sync.RWMutex
	data    map[string]string
	intData map[string]uint64
	events  []Event
}

func (r *Mock) notify(k *key) {
	k.mu.Lock()
	defer k.mu.Unlock()

	// Trigger all events
	r.eventHandles.mu.Lock()
	for _, event := range k.events {
		if e, ok := r.eventHandles.data[event]; ok {
			e.trigger()
		}
	}
	r.eventHandles.mu.Unlock()

	// Reset the list
	k.events = make([]Event, 0)
}

func (r *Mock) setValue(k *key, field, value string) {
	defer r.notify(k)

	k.mu.Lock()
	defer k.mu.Unlock()

	k.data[field] = value
}

func (*Mock) getValue(k *key, field string) (string, error) {
	k.mu.Lock()
	defer k.mu.Unlock()

	d, ok := k.data[field]
	if !ok {
		return d, ErrFieldNotExist
	}

	return d, nil
}

func (r *Mock) setIntValue(k *key, field string, value uint64) {
	defer r.notify(k)

	k.mu.Lock()
	defer k.mu.Unlock()

	k.intData[field] = value
}

func (*Mock) getIntValue(k *key, field string) (uint64, error) {
	k.mu.Lock()
	defer k.mu.Unlock()

	d, ok := k.intData[field]
	if !ok {
		return 0, ErrFieldNotExist
	}

	return d, nil
}

// keyHandle represents the object Win32 callers get when opening a key.
// Note that the Win32 API returns a HANDLE (i.e. a typedef'd void*), so this
// struct represents the structure that HANDLE points to.
type keyHandle struct {
	key      *key
	readOnly bool

	ctx       context.Context
	cancelCtx context.CancelFunc
}

// eventHandle represents the object Win32 callers get when creating an event.
// Note that the Win32 API returns a HANDLE (i.e. a typedef'd void*), so this
// struct represents the structure that HANDLE points to.
type eventHandle struct {
	ctx     context.Context
	trigger func()
}

// NewMock initializes a mocked registry.
func NewMock() *Mock {
	if !testing.Testing() {
		panic("This registry function should be used by tests only")
	}

	// We initialize the root and Software keys, as we consider that to be the minimal
	// "sane" Windows install.
	m := &Mock{
		ubuntuPro: key{
			mu:      &sync.RWMutex{},
			data:    make(map[string]string),
			intData: make(map[string]uint64),
			events:  make([]Event, 0),
		},
		ubuntu: key{
			mu:      &sync.RWMutex{},
			data:    make(map[string]string),
			intData: make(map[string]uint64),
			events:  make([]Event, 0),
		},
	}

	m.keyHandles.data = make(map[Key]*keyHandle)
	m.eventHandles.data = make(map[Event]*eventHandle)

	return m
}

// UbuntuProKeyExists returns whether the UbuntuPro key exists in the mock registry.
func (r *Mock) UbuntuProKeyExists() bool {
	r.ubuntuPro.mu.Lock()
	defer r.ubuntuPro.mu.Unlock()

	return r.keyExists
}

// RequireNoLeaks is a test helper to ensure we freed all allocations.
func (r *Mock) RequireNoLeaks(t *testing.T) {
	t.Helper()
	require.Empty(t, r.keyHandles.data, "registry mock: leaking registry key handles")
	require.Empty(t, r.eventHandles.data, "registry mock: leaking event handles")
}

// HKCUOpenKey mocks opening a key in the specified path under the HK_CURRENT_USER registry.
func (r *Mock) HKCUOpenKey(path string) (Key, error) {
	k := r.getKey(path)
	k.mu.Lock()
	defer k.mu.Unlock()

	if r.CannotOpen.Load() {
		return 0, ErrMock
	}

	return r.openKey(path, true), nil
}

// HKCUCreateKey opens a key in the specified path under the HK_CURRENT_USER registry with write permissions.
func (r *Mock) HKCUCreateKey(path string) (Key, error) {
	k := r.getKey(path)
	k.mu.Lock()
	defer k.mu.Unlock()

	if r.CannotCreate.Load() {
		return 0, ErrMock
	}

	if k == &r.ubuntuPro {
		r.keyExists = true
	}

	return r.openKey(path, false), nil
}

var validProPaths = []string{
	`Software\Canonical\UbuntuPro`,
	`Software/Canonical/UbuntuPro`,
}

var validUbuntuPaths = []string{
	`Software\Canonical\Ubuntu`,
	`Software/Canonical/Ubuntu`,
}

func (r *Mock) getKey(path string) *key {
	path = filepath.Clean(path)
	if slices.Contains(validProPaths, path) {
		return &r.ubuntuPro
	}
	if slices.Contains(validUbuntuPaths, path) {
		return &r.ubuntu
	}
	panic(fmt.Sprintf("Attempting to access key outside of UbuntuPro: %s", path))
}

func (r *Mock) openKey(path string, readOnly bool) Key {
	// The path validation is done in getKey
	return r.keyHandles.alloc(&keyHandle{
		key:      r.getKey(path),
		readOnly: readOnly,
	})
}

// CloseKey mocks releasing a key, triggering any associated events.
func (r *Mock) CloseKey(ptr Key) {
	defer r.keyHandles.free(ptr)

	r.keyHandles.mu.Lock()
	defer r.keyHandles.mu.Unlock()

	handle, ok := r.keyHandles.data[ptr]

	if !ok {
		return
	}

	if handle.cancelCtx != nil {
		handle.cancelCtx()
	}
}

// CloseEvent mocks releasing an event.
func (r *Mock) CloseEvent(ptr Event) {
	r.eventHandles.free(ptr)
}

// ReadValue returns the value of the specified field in the specified key.
func (r *Mock) ReadValue(ptr Key, field string) (value string, err error) {
	if ptr == 0 {
		return value, errors.New("null key")
	}

	if r.CannotRead.Load() {
		return "", ErrMock
	}

	handle, ok := r.keyHandles.data[ptr]

	if !ok {
		return "", ErrKeyNotExist
	}

	return r.getValue(handle.key, field)
}

// RegNotifyChangeKeyValue creates an event and attaches it to a registry key.
// Modifying that key or its children will trigger the event.
// This trigger can be detected by WaitForSingleObject.
func (r *Mock) RegNotifyChangeKeyValue(ptr Key) (Event, error) {
	if r.CannotWatch.Load() {
		return 0, ErrMock
	}

	r.keyHandles.mu.Lock()
	defer r.keyHandles.mu.Unlock()

	handle, ok := r.keyHandles.data[ptr]
	if !ok {
		return 0, ErrKeyNotExist
	}

	if handle.ctx != nil {
		return 0, errors.New("cannot have more than one listener per key handle")
	}

	handle.ctx, handle.cancelCtx = context.WithCancel(context.Background())

	// Create event
	evHandle := r.newEvent(handle.ctx)

	// Attach event to key
	handle.key.mu.Lock()
	defer handle.key.mu.Unlock()

	handle.key.events = append(handle.key.events, evHandle)

	return evHandle, nil
}

// WaitForSingleObject waits until the event is triggered. This is a blocking function.
func (r *Mock) WaitForSingleObject(handle Event) error {
	if r.CannotWait.Load() {
		return ErrMock
	}

	r.eventHandles.mu.Lock()
	event, ok := r.eventHandles.data[handle]
	r.eventHandles.mu.Unlock()

	if !ok {
		return errors.New("invalid event")
	}

	<-event.ctx.Done()
	return nil
}

// WriteValue is used to write a value into the registry.
func (r *Mock) WriteValue(ptr Key, field, value string, multiString bool) error {
	r.keyHandles.mu.Lock()
	defer r.keyHandles.mu.Unlock()

	handle, ok := r.keyHandles.data[ptr]

	if !ok {
		return ErrKeyNotExist
	}

	if handle.readOnly {
		return ErrAccessDenied
	}

	if strings.Contains(value, "\n") && !multiString {
		return fmt.Errorf("mock error: value contains newline, but multiString is false: %q", value)
	}

	r.setValue(handle.key, field, value)

	return nil
}

// ReadIntegerValue reads the value of the specified integer (DWORD or QWORD) field in the specified key.
func (r *Mock) ReadIntegerValue(ptr Key, field string) (uint64, error) {
	if ptr == 0 {
		return 0, errors.New("null key")
	}

	if r.CannotRead.Load() {
		return 0, ErrMock
	}

	handle, ok := r.keyHandles.data[ptr]

	if !ok {
		return 0, ErrKeyNotExist
	}

	return r.getIntValue(handle.key, field)
}

// SetDWordValue sets the value of the specified DWORD field in the specified key.
func (r *Mock) SetDWordValue(ptr Key, field string, value uint32) error {
	r.keyHandles.mu.Lock()
	defer r.keyHandles.mu.Unlock()

	handle, ok := r.keyHandles.data[ptr]

	if !ok {
		return ErrKeyNotExist
	}

	if handle.readOnly {
		return ErrAccessDenied
	}

	r.setIntValue(handle.key, field, uint64(value))

	return nil
}

func (r *Mock) newEvent(ctx context.Context) Event {
	ctx, cancel := context.WithCancel(ctx)

	return r.eventHandles.alloc(&eventHandle{
		ctx:     ctx,
		trigger: cancel,
	})
}

// Mocks memory access mapping a uintptr and real data.
type mockedHeap[KeyType ~uintptr, DataType any] struct {
	mu   sync.Mutex
	data map[KeyType]DataType
}

func (h *mockedHeap[P, D]) alloc(data D) P {
	h.mu.Lock()
	defer h.mu.Unlock()

	// Generate a random uintptr
	var ptr P
	for {
		//nolint:gosec // No need for a secure random number as this is test code
		ptr = P(rand.Int63())
		if ptr == 0 {
			continue
		}
		if _, ok := h.data[ptr]; ok {
			continue
		}
		break
	}

	h.data[ptr] = data
	return ptr
}

func (h *mockedHeap[P, D]) free(ptr P) {
	h.mu.Lock()
	defer h.mu.Unlock()

	delete(h.data, ptr)
}
