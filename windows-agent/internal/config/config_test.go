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
	untouched         registryState = iota // Nothing UbuntuPro-related exists, as though the program had never ran before
	keyExists                              // Key exists but is empty
	valuesExist                            // Key exists, token field exists but is empty
	valuesAreNotEmpty                      // Key exists, token field exists and is not empty
)

//nolint:dupl // This test setup is very similar to TestProToken, that is because they are both getters.
func TestProToken(t *testing.T) {
	t.Parallel()

	testCases := map[string]struct {
		mockErrors    uint32
		registryState registryState

		wantError bool
	}{
		"Success":                               {registryState: valuesAreNotEmpty},
		"Success when the key does not exist":   {registryState: untouched},
		"Success when the value does not exist": {registryState: keyExists},

		"Error when the registry key cannot be opened":    {registryState: valuesAreNotEmpty, mockErrors: registry.MockErrOnOpenKey, wantError: true},
		"Error when the registry key cannot be read from": {registryState: valuesAreNotEmpty, mockErrors: registry.MockErrReadValue, wantError: true},
	}

	for name, tc := range testCases {
		tc := tc
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			ctx := context.Background()

			r := registry.NewMock()
			conf := config.New(ctx, config.WithRegistry(r))

			r.Errors = tc.mockErrors
			if tc.registryState >= keyExists {
				r.KeyExists = true
			}
			if tc.registryState == valuesExist {
				r.UbuntuProData["ProToken"] = ""
			}
			if tc.registryState == valuesAreNotEmpty {
				r.UbuntuProData["ProToken"] = "EXAMPLE_TOKEN"
			}

			token, err := conf.ProToken(ctx)
			if tc.wantError {
				require.Error(t, err, "ProToken should return an error")
				return
			}
			require.NoError(t, err, "ProToken should return no error")

			// Test default values
			if tc.registryState < valuesAreNotEmpty {
				require.Equal(t, "", token, "Unexpected token value")
				return
			}

			// Test non-default values
			assert.Equal(t, "EXAMPLE_TOKEN", token, "Unexpected token value")
			assert.Zero(t, r.OpenKeyCount.Load(), "Leaking keys after ProToken")
		})
	}
}

//nolint:dupl // This test setup is very similar to TestProToken, that is because they are both getters.
func TestLandscapeURL(t *testing.T) {
	t.Parallel()

	testCases := map[string]struct {
		mockErrors    uint32
		registryState registryState

		wantError bool
	}{
		"Success":                               {registryState: valuesAreNotEmpty},
		"Success when the key does not exist":   {registryState: untouched},
		"Success when the value does not exist": {registryState: keyExists},

		"Error when the registry key cannot be opened":    {registryState: valuesAreNotEmpty, mockErrors: registry.MockErrOnOpenKey, wantError: true},
		"Error when the registry key cannot be read from": {registryState: valuesAreNotEmpty, mockErrors: registry.MockErrReadValue, wantError: true},
	}

	for name, tc := range testCases {
		tc := tc
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			ctx := context.Background()

			r := registry.NewMock()
			conf := config.New(ctx, config.WithRegistry(r))

			r.Errors = tc.mockErrors
			if tc.registryState >= keyExists {
				r.KeyExists = true
			}
			if tc.registryState == valuesExist {
				r.UbuntuProData["LandscapeURL"] = ""
			}
			if tc.registryState == valuesAreNotEmpty {
				r.UbuntuProData["LandscapeURL"] = "www.example.com/another-example"
			}

			landscapeURL, err := conf.LandscapeURL(ctx)
			if tc.wantError {
				require.Error(t, err, "LandscapeURL should return an error")
				return
			}
			require.NoError(t, err, "LandscapeURL should return no error")

			// Test default values
			if tc.registryState < valuesAreNotEmpty {
				require.Equal(t, "www.example.com", landscapeURL, "Unexpected Landscape URL value")
				return
			}

			// Test non-default values
			assert.Equal(t, "www.example.com/another-example", landscapeURL, "Unexpected Landscape URL value")
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
		"Success":                             {registryState: valuesAreNotEmpty},
		"Success when the key does not exist": {registryState: untouched},
		"Success when the pro token field does not exist": {registryState: keyExists},

		"Error when the registry key cannot be opened":    {registryState: valuesAreNotEmpty, mockErrors: registry.MockErrOnOpenKey, wantError: true},
		"Error when the registry key cannot be read from": {registryState: valuesAreNotEmpty, mockErrors: registry.MockErrReadValue, wantError: true},
	}

	for name, tc := range testCases {
		tc := tc
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			ctx := context.Background()

			r := registry.NewMock()
			conf := config.New(ctx, config.WithRegistry(r))

			r.Errors = tc.mockErrors
			if tc.registryState >= keyExists {
				r.KeyExists = true
			}
			if tc.registryState == valuesExist {
				r.UbuntuProData["ProToken"] = ""
			}
			if tc.registryState == valuesAreNotEmpty {
				r.UbuntuProData["ProToken"] = "EXAMPLE_TOKEN"
			}

			pt, err := conf.ProvisioningTasks(ctx)
			if tc.wantError {
				require.Error(t, err, "ProvisioningTasks should return an error")
				return
			}
			require.NoError(t, err, "ProvisioningTasks should return no error")

			var wantToken string
			if tc.registryState >= valuesAreNotEmpty {
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
		"Success":                             {registryState: valuesAreNotEmpty},
		"Success when the key does not exist": {registryState: untouched},
		"Success when the pro token field does not exist": {registryState: keyExists},

		"Error when the registry key cannot be written on due to lack of permission": {registryState: valuesAreNotEmpty, accessIsReadOnly: true, wantError: registry.ErrAccessDenied},
		"Error when the registry key cannot be opened":                               {registryState: valuesAreNotEmpty, mockErrors: registry.MockErrOnCreateKey, wantError: registry.ErrMock},
		"Error when the registry key cannot be written on":                           {registryState: valuesAreNotEmpty, mockErrors: registry.MockErrOnWriteValue, wantError: registry.ErrMock},
	}

	for name, tc := range testCases {
		tc := tc
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			ctx := context.Background()

			r := registry.NewMock()
			conf := config.New(ctx, config.WithRegistry(r))

			r.KeyIsReadOnly = tc.accessIsReadOnly
			r.Errors = tc.mockErrors
			var wantToken string
			if tc.registryState >= keyExists {
				r.KeyExists = true
			}
			if tc.registryState == valuesExist {
				r.UbuntuProData["ProToken"] = ""
			}
			if tc.registryState == valuesAreNotEmpty {
				wantToken = "ORIGINAL_TOKEN"
				r.UbuntuProData["ProToken"] = "ORIGINAL_TOKEN"
			}

			err := conf.SetProToken(ctx, "NEW_TOKEN")

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
