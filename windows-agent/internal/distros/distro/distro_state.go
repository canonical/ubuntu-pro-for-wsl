package distro

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/canonical/ubuntu-pro-for-windows/windows-agent/internal/distros/distro/touchdistro"
	wsl "github.com/ubuntu/gowsl"
)

// distroStateManager manages the state (running/stopped) of the distro with an internal counter.
// The distro is guaranteed to be running so long as the counter is above 0. This counter can
// be increased or decreased on demand, and is thread-safe.
type distroStateManager struct {
	refcount uint32
	cancel   func()
	mu       sync.Mutex

	distroIdentity identity
}

// State returns the state of the WSL distro, as implemeted by GoWSL.
func (m *distroStateManager) state() (s wsl.State, err error) {
	wslDistro, err := m.distroIdentity.getDistro()
	if err != nil {
		return s, err
	}

	return wslDistro.State()
}

// push increases the internal counter. If it was zero, the distro is awaken and locked awake.
// The context should be used to pass the GoWSL backend, and cancelling it does not override
// the need to call pop.
func (m *distroStateManager) push(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.refcount > 0 {
		s, err := m.state()
		if err != nil {
			return err
		}

		if s == wsl.Running {
			m.refcount++
			return nil
		}

		// Restart distro: need to re-call keepAwake
		m.cancel()
	}

	//nolint:staticcheck // False positive. 'cancel' is used in both paths.
	ctx, cancel := context.WithCancel(ctx)
	if err := m.keepAwake(ctx); err != nil {
		cancel()
		return err
	}

	m.refcount++
	m.cancel = cancel

	return nil
}

// pop decreases the internal counter. If it becomes zero, the distro awake lock is released.
func (m *distroStateManager) pop() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.refcount == 0 {
		return errors.New("excess calls to pop")
	}

	m.refcount--
	if m.refcount > 0 {
		return nil
	}

	m.cancel()
	m.cancel = nil

	return nil
}

func (m *distroStateManager) reset() {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.refcount == 0 {
		m.cancel = nil
		return
	}

	m.refcount = 0
	m.cancel()
	m.cancel = nil
}

// keepAwake ensures the distro is started by poking the distro every once in a while.
// Cancelling the context will remove this keep awake lock, but does not necessarily mean
// that the distribution will be shutdown right away.
//
// The distro will be running by the time keepAwake returns.
func (m *distroStateManager) keepAwake(ctx context.Context) (err error) {
	// Wake up distro
	if err := touchdistro.Touch(ctx, m.distroIdentity.Name); err != nil {
		return fmt.Errorf("could not wake distro up: %v", err)
	}

	// Keep distro awake
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case <-time.After(5 * time.Second):
				_ = touchdistro.Touch(ctx, m.distroIdentity.Name)
			}
		}
	}()

	return nil
}
