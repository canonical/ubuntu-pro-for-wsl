package registrywatcher_test

import (
	"context"
	"errors"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/canonical/ubuntu-pro-for-wsl/windows-agent/internal/config"
	"github.com/canonical/ubuntu-pro-for-wsl/windows-agent/internal/distros/database"
	"github.com/canonical/ubuntu-pro-for-wsl/windows-agent/internal/proservices/registrywatcher"
	"github.com/canonical/ubuntu-pro-for-wsl/windows-agent/internal/proservices/registrywatcher/registry"
	"github.com/stretchr/testify/require"
	wsl "github.com/ubuntu/gowsl"
	wslmock "github.com/ubuntu/gowsl/mock"
)

func TestRegistryWatcher(t *testing.T) {
	t.Parallel()

	const (
		defaultProToken        = "DefaultProToken"
		defaultLandscapeConfig = "DefaultLandscapeConfig"

		newProToken        = "NewProToken"
		newLandscapeConfig = "NewLandscapeConfig"
	)

	const maxUpdateTime = 5 * time.Second

	testCases := map[string]struct {
		startEmptyRegistry        bool
		breakCreateKey            bool
		breakOpenKey              bool
		breakReadValue            bool
		breakNotifyChangeKeyValue bool
		breakWaitForSingleObject  bool

		wantKeyNotExist bool
		wantCannotRead  bool
	}{
		"Success": {},
		"Success with an empty starting registry":                      {startEmptyRegistry: true},
		"Success with an empty starting registry and broken CreateKey": {startEmptyRegistry: true, breakCreateKey: true, wantKeyNotExist: true},

		"Success after not being able to open keys":       {breakOpenKey: true, wantCannotRead: true},
		"Success after not being able to read from keys":  {breakReadValue: true, wantCannotRead: true},
		"Success after not being able to watch keys":      {breakNotifyChangeKeyValue: true},
		"Success after not being able to wait for events": {breakWaitForSingleObject: true},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			ctx := context.Background()
			t.Parallel()
			if wsl.MockAvailable() {
				ctx = wsl.WithMock(ctx, wslmock.New())
			}

			conf := &mockConfig{}

			db, err := database.New(ctx, t.TempDir())
			require.NoError(t, err, "Setup: could not create empty DB")

			reg := registry.NewMock()
			defer reg.RequireNoLeaks(t)

			var startingProToken, startingLandscapeConfig string
			if !tc.startEmptyRegistry {
				startingProToken = defaultProToken
				startingLandscapeConfig = defaultLandscapeConfig

				func() {
					k, err := reg.HKCUCreateKey("Software/Canonical/UbuntuPro")
					require.NoError(t, err, "Setup: could not create key")
					defer reg.CloseKey(k)

					err = reg.WriteValue(k, "UbuntuProToken", startingProToken, false)
					require.NoError(t, err, "Setup: could not write UbuntuProToken into the registry")

					err = reg.WriteValue(k, "LandscapeConfig", startingLandscapeConfig, true)
					require.NoError(t, err, "Setup: could not write LandscapeConfig into the registry")
				}()
			}

			if tc.breakOpenKey {
				reg.CannotOpen.Store(true)
			}
			if tc.breakCreateKey {
				reg.CannotCreate.Store(true)
			}
			if tc.breakReadValue {
				reg.CannotRead.Store(true)
			}
			if tc.breakNotifyChangeKeyValue {
				reg.CannotWatch.Store(true)
			}
			if tc.breakWaitForSingleObject {
				reg.CannotWait.Store(true)
			}

			w := registrywatcher.New(ctx, conf, db, registrywatcher.WithRegistry(reg))
			w.Start()
			defer w.Stop()

			if tc.wantKeyNotExist {
				require.False(t, reg.UbuntuProKeyExists(), "UbuntuPro key should not exist after the watcher starts")
			} else {
				require.True(t, reg.UbuntuProKeyExists(), "UbuntuPro key should exist after the watcher starts")
			}

			if tc.wantCannotRead {
				// Cannot read from the registry: no data should be pushed
				require.True(t, conf.Empty(), "Registry watcher should not have updated the config")
				reg.CannotOpen.Store(false)
				reg.CannotRead.Store(false)
			} else {
				// Nothing broken: registry data is pushed during call to Start
				require.Eventually(t, func() bool {
					return conf.Received(config.RegistryData{
						UbuntuProToken:  startingProToken,
						LandscapeConfig: startingLandscapeConfig,
					})
				}, maxUpdateTime, 100*time.Millisecond, "Registry watcher should have updated the config")
			}

			if !tc.breakReadValue && !tc.breakOpenKey && !tc.breakNotifyChangeKeyValue && !tc.breakWaitForSingleObject {
				watchCtx, watchCancel := context.WithTimeout(ctx, maxUpdateTime)
				defer watchCancel()
				err = reg.WaitForWatch(watchCtx)
				require.NoError(t, err, "Registry watcher should have started watching")
			}

			if tc.breakCreateKey {
				// We disable the mock's broken CreateKey.
				// We need to do this because we need to pretend a user changed the registry.
				reg.CannotCreate.Store(false)
			}

			k, err := reg.HKCUCreateKey("Software/Canonical/UbuntuPro")
			require.NoError(t, err, "Setup: could not create key")
			defer reg.CloseKey(k)

			err = reg.WriteValue(k, "UbuntuProToken", newProToken, false)
			require.NoError(t, err, "Setup: could not write UbuntuProToken into the registry")

			require.Eventually(t, func() bool {
				return conf.Received(config.RegistryData{
					UbuntuProToken:  newProToken,
					LandscapeConfig: startingLandscapeConfig,
				})
			}, maxUpdateTime, 100*time.Millisecond, "Registry watcher should have updated the config after changing the registry")

			err = reg.WriteValue(k, "LandscapeConfig", newLandscapeConfig, true)
			require.NoError(t, err, "Setup: could not write LandscapeConfig into the registry")

			require.Eventually(t, func() bool {
				return conf.Received(config.RegistryData{
					UbuntuProToken:  newProToken,
					LandscapeConfig: newLandscapeConfig,
				})
			}, maxUpdateTime, 100*time.Millisecond, "Registry watcher should have updated the config after changing the registry")
		})
	}
}

func TestDefaultTelemetryConsent(t *testing.T) {
	t.Parallel()

	testCases := map[string]struct {
		preExistingKey   bool
		preExistingValue *uint32 // nil means value not present
		wantValue        uint64
	}{
		"Key missing": {
			preExistingKey: false,
			wantValue:      0,
		},
		"Key exists, value missing": {
			preExistingKey:   true,
			preExistingValue: nil,
			wantValue:        0,
		},
		"Key exists, value is 0": {
			preExistingKey:   true,
			preExistingValue: ptr(uint32(0)),
			wantValue:        0,
		},
		"Key exists, value is 1": {
			preExistingKey:   true,
			preExistingValue: ptr(uint32(1)),
			wantValue:        1,
		},
		"Key exists, value is 2": {
			preExistingKey:   true,
			preExistingValue: ptr(uint32(2)),
			wantValue:        0,
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			ctx := context.Background()
			t.Parallel()

			conf := &mockConfig{}
			db, err := database.New(ctx, t.TempDir())
			require.NoError(t, err)

			reg := registry.NewMock()
			defer reg.RequireNoLeaks(t)

			telemetryKeyPath := "Software/Canonical/Ubuntu"
			telemetryField := "UbuntuInsightsConsent"

			if tc.preExistingKey {
				k, err := reg.HKCUCreateKey(telemetryKeyPath)
				require.NoError(t, err)

				if tc.preExistingValue != nil {
					err = reg.SetDWordValue(k, telemetryField, *tc.preExistingValue)
					require.NoError(t, err)
				}
				reg.CloseKey(k)
			}

			w := registrywatcher.New(ctx, conf, db, registrywatcher.WithRegistry(reg))
			w.Start()
			w.Stop()

			// Check the value
			k, err := reg.HKCUOpenKey(telemetryKeyPath)
			require.NoError(t, err)
			defer reg.CloseKey(k)

			val, err := reg.ReadDWordValue(k, telemetryField)
			require.NoError(t, err)
			require.Equal(t, tc.wantValue, val)
		})
	}
}

func ptr[T any](v T) *T {
	return &v
}

type mockConfig struct {
	err      bool
	received []config.RegistryData

	mu sync.RWMutex
}

// UpdateRegistryData mocks the Config's method. It simply stores a history of the data it received.
func (conf *mockConfig) UpdateRegistryData(ctx context.Context, data config.RegistryData, db *database.DistroDB) error {
	if conf.err {
		return errors.New("mock conf error")
	}

	if db == nil {
		return errors.New("nil database")
	}

	conf.mu.Lock()
	defer conf.mu.Unlock()

	conf.received = append(conf.received, data)

	return nil
}

// Received returns whether the given config data was received.
func (conf *mockConfig) Received(data config.RegistryData) bool {
	conf.mu.RLock()
	defer conf.mu.RUnlock()

	return slices.Contains(conf.received, data)
}

// Empty returns whether no config data has been received.
func (conf *mockConfig) Empty() bool {
	conf.mu.RLock()
	defer conf.mu.RUnlock()

	return len(conf.received) == 0
}

func TestWaitForSingleObjectCancellation(t *testing.T) {
	t.Parallel()

	reg := registry.NewMock()
	defer reg.RequireNoLeaks(t)

	k, err := reg.HKCUCreateKey("Software/Canonical/UbuntuPro")
	require.NoError(t, err, "Setup: could not create key")
	defer reg.CloseKey(k)

	ev, err := reg.RegNotifyChangeKeyValue(k)
	require.NoError(t, err, "Setup: could not watch key")
	defer reg.CloseEvent(ev)

	w := registrywatcher.New(context.Background(), &mockConfig{}, nil, registrywatcher.WithRegistry(reg))

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err = w.WaitForSingleObject(ctx, ev)
	require.ErrorIs(t, err, context.Canceled, "WaitForSingleObject should return context.Canceled on cancellation")
}

func TestMockWaitForWatchTimeout(t *testing.T) {
	t.Parallel()

	reg := registry.NewMock()
	defer reg.RequireNoLeaks(t)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()

	err := reg.WaitForWatch(ctx)
	require.ErrorIs(t, err, context.DeadlineExceeded)
}

func TestMockConfigPredicates(t *testing.T) {
	t.Parallel()

	conf := &mockConfig{}
	require.True(t, conf.Empty())

	dummy := config.RegistryData{UbuntuProToken: "test-token"}
	require.False(t, conf.Received(dummy))

	db, err := database.New(context.Background(), t.TempDir())
	require.NoError(t, err)

	err = conf.UpdateRegistryData(context.Background(), dummy, db)
	require.NoError(t, err)

	require.False(t, conf.Empty())
	require.True(t, conf.Received(dummy))
}
