package registrywatcher_test

import (
	"context"
	"errors"
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

			// wantMsgLen is the expected number of times that data is sent to the config
			var wantMsgLen int

			if tc.wantCannotRead {
				// Cannot read from the registry: no data should be pushed
				time.Sleep(30 * time.Second)
				require.Equal(t, wantMsgLen, conf.ReceivedLen(), "Registry watcher should not have updated the config")
				reg.CannotOpen.Store(false)
				reg.CannotRead.Store(false)
			} else {
				// Nothing broken: registry data is pushed during call to Start
				wantMsgLen++
				require.Equal(t, wantMsgLen, conf.ReceivedLen(), "Registry watcher should have updated the config")
				require.Equal(t, startingProToken, conf.LatestReceived().UbuntuProToken, "Ubuntu Pro token config should have contained the registry value")
				require.Equal(t, startingLandscapeConfig, conf.LatestReceived().LandscapeConfig, "Landscape config should have contained the registry value")
			}

			// The watcher makes a redundant config push when it starts watching, except if readValue was broken.
			if !tc.breakReadValue {
				wantMsgLen++
				require.Eventually(t, func() bool { return conf.ReceivedLen() >= wantMsgLen },
					time.Minute, 100*time.Millisecond, "Registry watcher should have started watching")
				require.Equal(t, startingProToken, conf.LatestReceived().UbuntuProToken, "Ubuntu Pro token config should have contained the registry value")
				require.Equal(t, startingLandscapeConfig, conf.LatestReceived().LandscapeConfig, "Landscape config should have contained the registry value")
			}

			wantMsgLen = conf.ReceivedLen() + 1

			if tc.breakCreateKey {
				// We disable the mock's broken CreateKey.
				// We need to do this because we need to pretend a user changed the registry.
				reg.CannotCreate.Store(false)
			}

			k, err := reg.HKCUCreateKey("Software/Canonical/UbuntuPro")
			require.NoError(t, err, "Setup: could not create key")
			defer reg.CloseKey(k)

			wantMsgLen = conf.ReceivedLen() + 1
			err = reg.WriteValue(k, "UbuntuProToken", newProToken, false)
			require.NoError(t, err, "Setup: could not write UbuntuProToken into the registry")

			require.Eventuallyf(t, func() bool { return conf.ReceivedLen() >= wantMsgLen },
				maxUpdateTime, 100*time.Millisecond, "Registry watcher should have updated the config after changing the registry")
			require.Equal(t, newProToken, conf.LatestReceived().UbuntuProToken, "Ubuntu Pro token config should have contained the new registry value")
			require.Equal(t, startingLandscapeConfig, conf.LatestReceived().LandscapeConfig, "Landscape config should have contained the new registry value")

			wantMsgLen = conf.ReceivedLen() + 1
			err = reg.WriteValue(k, "LandscapeConfig", newLandscapeConfig, true)
			require.NoError(t, err, "Setup: could not write LandscapeConfig into the registry")

			require.Eventually(t, func() bool { return conf.ReceivedLen() >= wantMsgLen },
				maxUpdateTime, 100*time.Millisecond, "Registry watcher should have updated the config after changing the registry")
			require.Equal(t, newProToken, conf.LatestReceived().UbuntuProToken, "Ubuntu Pro token config should have contained the new registry value")
			require.Equal(t, newLandscapeConfig, conf.LatestReceived().LandscapeConfig, "Landscape config should have contained the new registry value")
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

// ReceivedLen is the number of times data has been pushed to the config.
func (conf *mockConfig) ReceivedLen() int {
	conf.mu.RLock()
	defer conf.mu.RUnlock()

	return len(conf.received)
}

// LatestReceived is the latest data pushed to the config.
func (conf *mockConfig) LatestReceived() config.RegistryData {
	conf.mu.RLock()
	defer conf.mu.RUnlock()

	return conf.received[len(conf.received)-1]
}
