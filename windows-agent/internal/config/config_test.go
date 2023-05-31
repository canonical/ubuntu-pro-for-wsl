package config_test

import (
	"context"
	"testing"

	config "github.com/canonical/ubuntu-pro-for-windows/windows-agent/internal/config"
	"github.com/canonical/ubuntu-pro-for-windows/windows-agent/internal/config/registry"
	"github.com/canonical/ubuntu-pro-for-windows/windows-agent/internal/distros/task"
	"github.com/canonical/ubuntu-pro-for-windows/windows-agent/internal/tasks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// registryState represents how much data is in the registry.
type registryState int

const (
	untouched          registryState = iota // Nothing UbuntuPro-related exists, as though the program had never ran before
	keyExists                               // Key exists but is empty
	tokenFieldExists                        // Key exists, token field exists but is empty
	tokenFieldHasValue                      // Key exists, token field exists and is not empty
)

func TestNewAndGetters(t *testing.T) {
	ctx := context.Background()

	conf, err := config.New(ctx, config.WithRegistry(registry.NewMock()))
	require.NoError(t, err, "New should not return an error")

	require.NotEmpty(t, conf.Hostname(), "Hostname should not be an empty string")
	require.NotEmpty(t, conf.Username(), "Username should not be an empty string")
	require.NotEmpty(t, conf.Pseudonym(), "Pseudonym should not be an empty string")
}

func TestProToken(t *testing.T) {
	t.Parallel()

	testCases := map[string]struct {
		mockErrors    uint32
		registryState registryState

		wantError bool
	}{
		"Success":                             {registryState: tokenFieldHasValue},
		"Success when the key does not exist": {registryState: untouched},
		"Success when the pro token field does not exist": {registryState: keyExists},

		"Error when the registry key cannot be opened":    {registryState: tokenFieldHasValue, mockErrors: registry.MockErrOnOpenKey, wantError: true},
		"Error when the registry key cannot be read from": {registryState: tokenFieldHasValue, mockErrors: registry.MockErrReadValue, wantError: true},
	}

	for name, tc := range testCases {
		tc := tc
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			ctx := context.Background()

			r := registry.NewMock()
			conf, err := config.New(ctx, config.WithRegistry(r))
			require.NoError(t, err, "Setup: could not initialize Config")

			r.Errors = tc.mockErrors
			if tc.registryState >= keyExists {
				r.KeyExists = true
			}
			if tc.registryState == tokenFieldExists {
				r.UbuntuProData["ProToken"] = ""
			}
			if tc.registryState == tokenFieldHasValue {
				r.UbuntuProData["ProToken"] = "EXAMPLE_TOKEN"
			}

			token, err := conf.ProToken(ctx)
			if tc.wantError {
				require.Error(t, err, "ProToken should return an error")
				return
			}
			require.NoError(t, err, "ProToken should return no error")

			if tc.registryState < tokenFieldHasValue {
				require.Equal(t, token, "", "Unexpected token value")
				return
			}

			require.Equal(t, token, "EXAMPLE_TOKEN", "Unexpected token value")
			assert.Zero(t, r.OpenKeyCount.Load(), "Leaking keys after ProToken")
		})
	}
}

func TestProvisioningTasks(t *testing.T) {
	t.Parallel()

	testCases := map[string]struct {
		mockErrors    uint32
		registryState registryState

		wantError bool
	}{
		"Success":                             {registryState: tokenFieldHasValue},
		"Success when the key does not exist": {registryState: untouched},
		"Success when the pro token field does not exist": {registryState: keyExists},

		"Error when the registry key cannot be opened":    {registryState: tokenFieldHasValue, mockErrors: registry.MockErrOnOpenKey, wantError: true},
		"Error when the registry key cannot be read from": {registryState: tokenFieldHasValue, mockErrors: registry.MockErrReadValue, wantError: true},
	}

	for name, tc := range testCases {
		tc := tc
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			ctx := context.Background()

			r := registry.NewMock()
			conf, err := config.New(ctx, config.WithRegistry(r))
			require.NoError(t, err, "Setup: could not initialize Config")

			r.Errors = tc.mockErrors
			if tc.registryState >= keyExists {
				r.KeyExists = true
			}
			if tc.registryState == tokenFieldExists {
				r.UbuntuProData["ProToken"] = ""
			}
			if tc.registryState == tokenFieldHasValue {
				r.UbuntuProData["ProToken"] = "EXAMPLE_TOKEN"
			}

			pt, err := conf.ProvisioningTasks(ctx)
			if tc.wantError {
				require.Error(t, err, "ProvisioningTasks should return an error")
				return
			}
			require.NoError(t, err, "ProvisioningTasks should return no error")

			var wantToken string
			if tc.registryState >= tokenFieldHasValue {
				wantToken = "EXAMPLE_TOKEN"
			}

			require.ElementsMatch(t, pt, []task.Task{
				tasks.ProAttachment{Token: wantToken},
			}, "Unexpected contents returned by ProvisioningTasks")
		})
	}
}

func TestSetProToken(t *testing.T) {
	t.Parallel()

	testCases := map[string]struct {
		mockErrors       uint32
		registryState    registryState
		accessIsReadOnly bool

		wantError error
	}{
		"Success":                             {registryState: tokenFieldHasValue},
		"Success when the key does not exist": {registryState: untouched},
		"Success when the pro token field does not exist": {registryState: keyExists},

		"Error when the registry key cannot be written on due to lack of permission": {registryState: tokenFieldHasValue, accessIsReadOnly: true, wantError: registry.ErrAccessDenied},
		"Error when the registry key cannot be opened":                               {registryState: tokenFieldHasValue, mockErrors: registry.MockErrOnCreateKey, wantError: registry.ErrMock},
		"Error when the registry key cannot be written on":                           {registryState: tokenFieldHasValue, mockErrors: registry.MockErrOnWriteValue, wantError: registry.ErrMock},
	}

	for name, tc := range testCases {
		tc := tc
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			ctx := context.Background()

			r := registry.NewMock()
			conf, err := config.New(ctx, config.WithRegistry(r))
			require.NoError(t, err, "Setup: could not initialize Config")

			r.KeyIsReadOnly = tc.accessIsReadOnly
			r.Errors = tc.mockErrors
			var wantToken string
			if tc.registryState >= keyExists {
				r.KeyExists = true
			}
			if tc.registryState == tokenFieldExists {
				r.UbuntuProData["ProToken"] = ""
			}
			if tc.registryState == tokenFieldHasValue {
				wantToken = "ORIGINAL_TOKEN"
				r.UbuntuProData["ProToken"] = "ORIGINAL_TOKEN"
			}

			err = conf.SetProToken(ctx, "NEW_TOKEN")

			if tc.wantError != nil {
				require.Error(t, err, "ProvisioningTasks should return an error")
				require.ErrorIs(t, err, tc.wantError, "ProvisioningTasks returned an error of unexpected type")
			} else {
				require.NoError(t, err, "ProvisioningTasks should return no error")
				wantToken = "NEW_TOKEN"
			}

			// Disable errors so we can retrieve the token
			r.Errors = 0
			token, err := conf.ProToken(ctx)
			require.NoError(t, err, "ProToken should return no error")

			require.Equal(t, wantToken, token, "ProToken returned an unexpected value for the token")
		})
	}
}
