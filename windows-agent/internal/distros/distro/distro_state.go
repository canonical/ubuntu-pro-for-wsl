package distro

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/canonical/ubuntu-pro-for-windows/windows-agent/internal/distros/distro/touchdistro"
	log "github.com/canonical/ubuntu-pro-for-windows/windows-agent/internal/grpc/logstreamer"
	wsl "github.com/ubuntu/gowsl"
)

// stateManager manages the state (running/stopped) of the distro with an internal counter.
// The distro is guaranteed to be running so long as the counter is above 0. This counter can
// be increased or decreased on demand, and is thread-safe.
type stateManager struct {
	refcount uint32
	cancel   func()
	mu       sync.Mutex

	distroIdentity identity
}

// State returns the state of the WSL distro, as implemeted by GoWSL.
func (m *stateManager) state() (s wsl.State, err error) {
	wslDistro, err := m.distroIdentity.getDistro()
	if err != nil {
		return s, err
	}

	return wslDistro.State()
}

// lock increases the internal counter. If it was zero, the distro is awaken and locked awake.
// The context should be used to pass the GoWSL backend, and cancelling it does not override
// the need to call unlock.
//
//nolint:nolintlint  // Golangci-lint gives false positives only without --build-tags=gowslmock
func (m *stateManager) lock(ctx context.Context) error {
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

// release decreases the internal counter. If it becomes zero, the distro awake lock is released.
func (m *stateManager) release() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.refcount == 0 {
		return errors.New("excess calls to release")
	}

	m.refcount--
	if m.refcount > 0 {
		return nil
	}

	m.cancel()
	m.cancel = nil

	return nil
}

// reset returns the count back to zero. Equivalent to unlocking all standing locks.
func (m *stateManager) reset() {
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
func (m *stateManager) keepAwake(ctx context.Context) (err error) {
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
				if err := touchdistro.Touch(ctx, m.distroIdentity.Name); err != nil {
					log.Errorf(ctx, "Distro %q: %v", m.distroIdentity.Name, err)
				}
			}
		}
	}()

	return nil
}
