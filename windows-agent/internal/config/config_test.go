package config_test

import (
	"context"
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	config "github.com/canonical/ubuntu-pro-for-wsl/windows-agent/internal/config"
	"github.com/canonical/ubuntu-pro-for-wsl/windows-agent/internal/distros/database"
	"github.com/canonical/ubuntu-pro-for-wsl/windows-agent/internal/distros/task"
	"github.com/canonical/ubuntu-pro-for-wsl/windows-agent/internal/tasks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	wsl "github.com/ubuntu/gowsl"
	wslmock "github.com/ubuntu/gowsl/mock"
	"gopkg.in/yaml.v3"
)

// settingsState represents how much data is in the registry.
type settingsState uint64

const (
	untouched  settingsState = 0 // Nothing UbuntuPro-related exists, as though the program had never ran before
	fileExists settingsState = 1 // File exists but is empty

	// Registry settings.
	orgTokenHasValue           = 1 << 2 // Organization token is not empty
	orgLandscapeConfigHasValue = 1 << 3 // Organization landscape config is not empty

	// File settings.
	userTokenExists           = fileExists | 1<<(4+iota) // File exists, user token exists
	storeTokenExists                                     // File exists, microsoft store token exists
	userLandscapeConfigExists                            // File exists, landscape client config exists
	landscapeUIDExists                                   // File exists, landscape agent UID exists

	userTokenHasValue           = userTokenExists | 1<<20           // File exists, user token exists, and is not empty
	storeTokenHasValue          = storeTokenExists | 1<<21          // File exists, microsoft store token exists, and is not empty
	userLandscapeConfigHasValue = userLandscapeConfigExists | 1<<22 // File exists, landscape client config exists, and is not empty
	landscapeUIDHasValue        = landscapeUIDExists | 1<<23        // File exists, landscape agent UID exists, and is not empty
)

func TestSubscription(t *testing.T) {
	if wsl.MockAvailable() {
		t.Parallel()
	}

	testCases := map[string]struct {
		breakFile     bool
		settingsState settingsState

		wantToken  string
		wantSource config.Source
		wantError  bool
	}{
		"Success": {settingsState: userTokenHasValue, wantToken: "user_token", wantSource: config.SourceUser},
		"Success when neither registry settings nor conf file exist": {settingsState: untouched},

		"Success when there is an organization token": {settingsState: orgTokenHasValue, wantToken: "org_token", wantSource: config.SourceRegistry},
		"Success when there is a user token":          {settingsState: userTokenHasValue, wantToken: "user_token", wantSource: config.SourceUser},
		"Success when there is a store token":         {settingsState: storeTokenHasValue, wantToken: "store_token", wantSource: config.SourceMicrosoftStore},

		"Success when an organization token shadows a user token":                           {settingsState: orgTokenHasValue | userTokenHasValue, wantToken: "org_token", wantSource: config.SourceRegistry},
		"Success when an organization token shadows a store token":                          {settingsState: orgTokenHasValue | storeTokenHasValue, wantToken: "org_token", wantSource: config.SourceRegistry},
		"Success when a store token shadows a user token":                                   {settingsState: userTokenHasValue | storeTokenHasValue, wantToken: "store_token", wantSource: config.SourceMicrosoftStore},
		"Success when an organization token shadows a user token, and an empty store token": {settingsState: orgTokenHasValue | userTokenHasValue | storeTokenExists, wantToken: "org_token", wantSource: config.SourceRegistry},

		"Error when the file cannot be read from": {settingsState: untouched, breakFile: true, wantError: true},
	}

	//nolint: dupl // This is mostly duplicate but de-duplicating with a meta-test worsens readability
	for name, tc := range testCases {
		tc := tc
		t.Run(name, func(t *testing.T) {
			ctx := context.Background()
			if wsl.MockAvailable() {
				t.Parallel()
				ctx = wsl.WithMock(ctx, wslmock.New())
			}

			db, err := database.New(ctx, t.TempDir(), nil)
			require.NoError(t, err, "Setup: could not create empty database")

			setup, dir := setUpMockSettings(t, ctx, db, tc.settingsState, tc.breakFile, false)
			conf := config.New(ctx, dir)
			setup(t, conf)

			token, source, err := conf.Subscription()
			if tc.wantError {
				require.Error(t, err, "ProToken should return an error")
				return
			}
			require.NoError(t, err, "ProToken should return no error")

			// Test values
			require.Equal(t, tc.wantToken, token, "Unexpected token value")
			require.Equal(t, tc.wantSource, source, "Unexpected token source")
		})
	}
}

func TestLandscapeConfig(t *testing.T) {
	if wsl.MockAvailable() {
		t.Parallel()
	}

	testCases := map[string]struct {
		breakFile     bool
		settingsState settingsState

		wantLandscapeConfig string
		wantSource          config.Source
		wantError           bool
	}{
		"Success": {settingsState: userLandscapeConfigHasValue, wantLandscapeConfig: "[client]\nuser=JohnDoe", wantSource: config.SourceUser},

		"Success when neither registry data nor conf file exist": {settingsState: untouched},

		"Success when there is an organization conf": {settingsState: orgLandscapeConfigHasValue, wantLandscapeConfig: "[client]\nuser=BigOrg", wantSource: config.SourceRegistry},
		"Success when there is a user conf":          {settingsState: userLandscapeConfigHasValue, wantLandscapeConfig: "[client]\nuser=JohnDoe", wantSource: config.SourceUser},

		"Success when an organization config shadows a user config": {settingsState: orgLandscapeConfigHasValue | userLandscapeConfigHasValue, wantLandscapeConfig: "[client]\nuser=BigOrg", wantSource: config.SourceRegistry},

		"Error when the file cannot be read from": {settingsState: untouched, breakFile: true, wantError: true},
	}

	//nolint: dupl // This is mostly duplicate but de-duplicating with a meta-test worsens readability
	for name, tc := range testCases {
		tc := tc
		t.Run(name, func(t *testing.T) {
			ctx := context.Background()
			if wsl.MockAvailable() {
				t.Parallel()
				ctx = wsl.WithMock(ctx, wslmock.New())
			}

			db, err := database.New(ctx, t.TempDir(), nil)
			require.NoError(t, err, "Setup: could not create empty database")

			setup, dir := setUpMockSettings(t, ctx, db, tc.settingsState, tc.breakFile, false)
			conf := config.New(ctx, dir)
			setup(t, conf)

			landscapeConf, source, err := conf.LandscapeClientConfig()
			if tc.wantError {
				require.Error(t, err, "ProToken should return an error")
				return
			}
			require.NoError(t, err, "ProToken should return no error")

			// Test values
			require.Equal(t, tc.wantLandscapeConfig, landscapeConf, "Unexpected token value")
			require.Equal(t, tc.wantSource, source, "Unexpected token source")
		})
	}
}

func TestLandscapeAgentUID(t *testing.T) {
	if wsl.MockAvailable() {
		t.Parallel()
	}

	testCases := map[string]struct {
		settingsState     settingsState
		breakFile         bool
		breakFileContents bool

		wantError bool
	}{
		"Success":                               {settingsState: landscapeUIDHasValue},
		"Success when the file does not exist":  {settingsState: untouched},
		"Success when the value does not exist": {settingsState: fileExists},

		"Error when the file cannot be opened": {settingsState: fileExists, breakFile: true, wantError: true},
		"Error when the file cannot be parsed": {settingsState: fileExists, breakFileContents: true, wantError: true},
	}

	for name, tc := range testCases {
		tc := tc
		t.Run(name, func(t *testing.T) {
			ctx := context.Background()
			if wsl.MockAvailable() {
				t.Parallel()
				ctx = wsl.WithMock(ctx, wslmock.New())
			}

			db, err := database.New(ctx, t.TempDir(), nil)
			require.NoError(t, err, "Setup: could not create empty database")

			setup, dir := setUpMockSettings(t, ctx, db, tc.settingsState, tc.breakFile, false)
			if tc.breakFileContents {
				err := os.WriteFile(filepath.Join(dir, "config"), []byte("\tmessage:\n\t\tthis is not YAML!["), 0600)
				require.NoError(t, err, "Setup: could not re-write config file")
			}

			conf := config.New(ctx, dir)
			setup(t, conf)

			v, err := conf.LandscapeAgentUID()
			if tc.wantError {
				require.Error(t, err, "LandscapeAgentUID should return an error")
				return
			}
			require.NoError(t, err, "LandscapeAgentUID should return no error")

			// Test default values
			if !tc.settingsState.is(landscapeUIDHasValue) {
				require.Emptyf(t, v, "Unexpected value when LandscapeAgentUID is not set in registry")
				return
			}

			// Test non-default values
			assert.Equal(t, "landscapeUID1234", v, "LandscapeAgentUID returned an unexpected value")
		})
	}
}

func TestProvisioningTasks(t *testing.T) {
	if wsl.MockAvailable() {
		t.Parallel()
	}

	testCases := map[string]struct {
		settingsState settingsState

		wantToken         string
		wantLandscapeConf string
		wantLandscapeUID  string

		wantError bool
	}{
		"Success when there is no data":                               {settingsState: untouched},
		"Success when there is an empty config file":                  {settingsState: fileExists},
		"Success when the file's pro token field exists but is empty": {settingsState: userTokenExists},
		"Success with a user token":                                   {settingsState: userTokenHasValue, wantToken: "user_token"},
		"Success when there is Landscape config":                      {settingsState: userLandscapeConfigHasValue | landscapeUIDHasValue, wantLandscapeConf: "[client]\nuser=JohnDoe", wantLandscapeUID: "landscapeUID1234"},
	}

	for name, tc := range testCases {
		tc := tc
		t.Run(name, func(t *testing.T) {
			ctx := context.Background()
			if wsl.MockAvailable() {
				t.Parallel()
				ctx = wsl.WithMock(ctx, wslmock.New())
			}

			db, err := database.New(ctx, t.TempDir(), nil)
			require.NoError(t, err, "Setup: could not create empty database")

			setup, dir := setUpMockSettings(t, ctx, db, tc.settingsState, false, false)
			conf := config.New(ctx, dir)
			setup(t, conf)

			gotTasks, err := conf.ProvisioningTasks(ctx, "UBUNTU")
			if tc.wantError {
				require.Error(t, err, "ProvisioningTasks should return an error")
				return
			}
			require.NoError(t, err, "ProvisioningTasks should return no error")

			wantTasks := []task.Task{
				tasks.ProAttachment{Token: tc.wantToken},
				tasks.LandscapeConfigure{
					Config:       tc.wantLandscapeConf,
					HostagentUID: tc.wantLandscapeUID,
				},
			}

			require.ElementsMatch(t, wantTasks, gotTasks, "Unexpected contents returned by ProvisioningTasks")
		})
	}
}

func TestSetUserSubscription(t *testing.T) {
	if wsl.MockAvailable() {
		t.Parallel()
	}

	testCases := map[string]struct {
		settingsState   settingsState
		breakFile       bool
		cannotWriteFile bool
		emptyToken      bool

		want      string
		wantError bool
	}{
		"Success":                          {settingsState: userTokenHasValue, want: "new_token"},
		"Success disabling a subscription": {settingsState: userTokenHasValue, emptyToken: true, want: ""},

		"Error when there is a store token active": {settingsState: storeTokenHasValue, wantError: true},
		"Error when the file cannot be opened":     {settingsState: fileExists, breakFile: true, wantError: true},
		"Error when the file cannot be written":    {settingsState: fileExists, cannotWriteFile: true, wantError: true},
	}

	//nolint:dupl // This is mostly duplicate with TestSetStoreConfig but de-duplicating with a meta-test worsens readability
	for name, tc := range testCases {
		tc := tc
		t.Run(name, func(t *testing.T) {
			ctx := context.Background()
			if wsl.MockAvailable() {
				t.Parallel()
				ctx = wsl.WithMock(ctx, wslmock.New())
			}

			db, err := database.New(ctx, t.TempDir(), nil)
			require.NoError(t, err, "Setup: could not create empty database")

			setup, dir := setUpMockSettings(t, ctx, db, tc.settingsState, tc.breakFile, tc.cannotWriteFile)
			conf := config.New(ctx, dir)
			setup(t, conf)

			token := "new_token"
			if tc.emptyToken {
				token = ""
			}

			var calledProNotifier int
			conf.SetUbuntuProNotifier(func(context.Context, string) {
				calledProNotifier++
			})

			conf.SetLandscapeNotifier(func(context.Context, string, string) {
				require.Fail(t, "LandscapeNotifier should not be called")
			})

			err = conf.SetUserSubscription(ctx, token)
			if tc.wantError {
				require.Error(t, err, "SetSubscription should return an error")
				return
			}
			require.NoError(t, err, "SetSubscription should return no error")

			require.Equal(t, 1, calledProNotifier, "ProNotifier should have been called once")

			got, _, err := conf.Subscription()
			require.NoError(t, err, "ProToken should return no error")

			require.Equal(t, tc.want, got, "ProToken returned an unexpected value for the token")

			// Set the same token again
			calledProNotifier = 0
			err = conf.SetUserSubscription(ctx, token)
			require.NoError(t, err, "SetUserSubscription should return no error")
			require.Zero(t, calledProNotifier, "ProNotifier should not have been called again")
		})
	}
}

func TestSetStoreSubscription(t *testing.T) {
	if wsl.MockAvailable() {
		t.Parallel()
	}

	testCases := map[string]struct {
		settingsState   settingsState
		breakFile       bool
		cannotWriteFile bool
		emptyToken      bool

		want      string
		wantError bool
	}{
		"Success":                          {settingsState: userTokenHasValue, want: "new_token"},
		"Success disabling a subscription": {settingsState: storeTokenHasValue, emptyToken: true, want: ""},
		"Success overriding an existing store token": {settingsState: storeTokenHasValue, want: "new_token"},

		"Error when the file cannot be opened":  {settingsState: fileExists, breakFile: true, wantError: true},
		"Error when the file cannot be written": {settingsState: fileExists, cannotWriteFile: true, wantError: true},
	}

	//nolint:dupl // This is mostly duplicate with TestSetUserConfig but de-duplicating with a meta-test worsens readability
	for name, tc := range testCases {
		tc := tc
		t.Run(name, func(t *testing.T) {
			ctx := context.Background()
			if wsl.MockAvailable() {
				t.Parallel()
				ctx = wsl.WithMock(ctx, wslmock.New())
			}

			db, err := database.New(ctx, t.TempDir(), nil)
			require.NoError(t, err, "Setup: could not create empty database")

			setup, dir := setUpMockSettings(t, ctx, db, tc.settingsState, tc.breakFile, tc.cannotWriteFile)
			conf := config.New(ctx, dir)
			setup(t, conf)

			token := "new_token"
			if tc.emptyToken {
				token = ""
			}

			var calledProNotifier int
			conf.SetUbuntuProNotifier(func(context.Context, string) {
				calledProNotifier++
			})

			conf.SetLandscapeNotifier(func(context.Context, string, string) {
				require.Fail(t, "LandscapeNotifier should not be called")
			})

			err = conf.SetStoreSubscription(ctx, token)
			if tc.wantError {
				require.Error(t, err, "SetSubscription should return an error")
				return
			}
			require.NoError(t, err, "SetSubscription should return no error")

			require.Equal(t, 1, calledProNotifier, "ProNotifier should have been called once")

			got, _, err := conf.Subscription()
			require.NoError(t, err, "ProToken should return no error")

			require.Equal(t, tc.want, got, "ProToken returned an unexpected value for the token")

			// Set the same token again
			calledProNotifier = 0
			err = conf.SetStoreSubscription(ctx, token)
			require.NoError(t, err, "SetStoreSubscription should return no error")
			require.Zero(t, calledProNotifier, "ProNotifier should not have been called again")
		})
	}
}

func TestSetUserLandscapeConfig(t *testing.T) {
	t.Parallel()

	testCases := map[string]struct {
		settingsState settingsState
		breakFile bool

		wantError bool
	}{
		"Success": {settingsState: untouched},

		"Error when an organization landscape config is already set": {settingsState: orgLandscapeConfigHasValue, wantError: true},
		"Error when an configuration cannot be read": {settingsState: untouched, breakFile: true, wantError: true},
	}

	for name, tc := range testCases {
		tc := tc

		t.Run(name, func(t *testing.T) {
			ctx := context.Background()
			if wsl.MockAvailable() {
				t.Parallel()
				ctx = wsl.WithMock(ctx, wslmock.New())
			}

			db, err := database.New(ctx, t.TempDir(), nil)
			require.NoError(t, err, "Setup: could not create empty database")

			setup, dir := setUpMockSettings(t, ctx, db, tc.settingsState, tc.breakFile, false)
			conf := config.New(ctx, dir)
			setup(t, conf)

			landscapeConfig := "LANDSCAPE CONFIG"

			err = conf.SetUserLandscapeConfig(ctx, landscapeConfig)
			if tc.wantError {
				require.Error(t, err, "SetUserLandscapeConfig should return an error")
				return
			}
			require.NoError(t, err, "SetUserLandscapeConfig should return no errors")

			got, src, err := conf.LandscapeClientConfig()
			require.NoError(t, err, "LandscapeClientConfig should return no errors")
			require.Equal(t, landscapeConfig, got, "Did not get the same value for landscape config as we set")
			require.Equal(t, config.SourceUser, src, "Did not get the same value for landscape config as we set")
		})
	}
}

func TestSetLandscapeAgentUID(t *testing.T) {
	if wsl.MockAvailable() {
		t.Parallel()
	}

	testCases := map[string]struct {
		settingsState   settingsState
		emptyUID        bool
		breakFile       bool
		cannotWriteFile bool

		want      string
		wantError bool
	}{
		"Success overriding the UID":                      {settingsState: landscapeUIDHasValue, want: "new_uid"},
		"Success unsetting the UID":                       {settingsState: landscapeUIDHasValue, emptyUID: true, want: ""},
		"Success when the file does not exist":            {settingsState: untouched, want: "new_uid"},
		"Success when the pro token field does not exist": {settingsState: fileExists, want: "new_uid"},

		"Error when the file cannot be opened":  {settingsState: landscapeUIDHasValue, breakFile: true, want: "landscapeUID1234", wantError: true},
		"Error when the file cannot be written": {settingsState: fileExists, cannotWriteFile: true, wantError: true},
	}

	for name, tc := range testCases {
		tc := tc
		t.Run(name, func(t *testing.T) {
			ctx := context.Background()
			if wsl.MockAvailable() {
				t.Parallel()
				ctx = wsl.WithMock(ctx, wslmock.New())
			}

			db, err := database.New(ctx, t.TempDir(), nil)
			require.NoError(t, err, "Setup: could not create empty database")

			setup, dir := setUpMockSettings(t, ctx, db, tc.settingsState, tc.breakFile, tc.cannotWriteFile)
			conf := config.New(ctx, dir)
			setup(t, conf)

			uid := "new_uid"
			if tc.emptyUID {
				uid = ""
			}

			conf.SetUbuntuProNotifier(func(context.Context, string) {
				require.Fail(t, "UbuntuProNotifier should not be called")
			})

			conf.SetLandscapeNotifier(func(context.Context, string, string) {
				require.Fail(t, "LandscapeNotifier should not be called")
			})

			err = conf.SetLandscapeAgentUID(uid)
			if tc.wantError {
				require.Error(t, err, "SetLandscapeAgentUID should return an error")
				return
			}
			require.NoError(t, err, "SetLandscapeAgentUID should return no error")

			got, err := conf.LandscapeAgentUID()
			require.NoError(t, err, "LandscapeAgentUID should return no error")

			require.Equal(t, tc.want, got, "LandscapeAgentUID returned an unexpected value for the token")
		})
	}
}

func TestUpdateRegistryData(t *testing.T) {
	if wsl.MockAvailable() {
		t.Parallel()
	}

	//nolint:gosec // These are not real credentials
	const (
		proToken1      = "UBUNTU_PRO_TOKEN_FIRST"
		landscapeConf1 = "[client]greeting=hello"

		proToken2      = "UBUNTU_PRO_TOKEN_SECOND"
		landscapeConf2 = "[client]greeting=cheers"
	)

	testCases := map[string]struct {
		settingsState   settingsState
		breakConfigFile bool

		wantErr bool
	}{
		"Success":                        {},
		"Success overriding user config": {settingsState: userTokenHasValue | userLandscapeConfigHasValue},

		"Error when we cannot load from file": {breakConfigFile: true, wantErr: true},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			ctx := context.Background()
			if wsl.MockAvailable() {
				t.Parallel()
				ctx = wsl.WithMock(ctx, wslmock.New())
			}

			db, err := database.New(ctx, t.TempDir(), nil)
			require.NoError(t, err, "Setup: could not create empty database")

			_, dir := setUpMockSettings(t, ctx, db, tc.settingsState, tc.breakConfigFile, false)
			c := config.New(ctx, dir)

			var calledUbuntuProNotifier int
			c.SetUbuntuProNotifier(func(context.Context, string) {
				calledUbuntuProNotifier++
			})

			var calledLandscapeNotifier int
			c.SetLandscapeNotifier(func(context.Context, string, string) {
				calledLandscapeNotifier++
			})

			// Enter a first set of data to override the defaults
			err = c.UpdateRegistryData(ctx, config.RegistryData{
				UbuntuProToken:  proToken1,
				LandscapeConfig: landscapeConf1,
			}, db)
			if tc.wantErr {
				require.Error(t, err, "UpdateRegistryData should have failed")
				return
			}
			require.NoError(t, err, "UpdateRegistryData should not have failed")

			tokenCsum1, lcapeCsum1 := loadChecksums(t, dir)
			require.NotEmpty(t, tokenCsum1, "Subscription checksum should not be empty")
			require.NotEmpty(t, lcapeCsum1, "Landscape checksum should not be empty")

			require.Equal(t, 1, calledUbuntuProNotifier, "UbuntuProNotifier called an unexpected amount of times")
			require.Equal(t, 1, calledLandscapeNotifier, "LandscapeNotifier called an unexpected amount of times")
			calledUbuntuProNotifier = 0
			calledLandscapeNotifier = 0

			token, src, err := c.Subscription()
			require.NoError(t, err, "Subscription should not return any errors")
			require.Equal(t, proToken1, token, "Subscription did not return the token we wrote")
			require.Equal(t, config.SourceRegistry, src, "Subscription did not come from registry")

			lcape, src, err := c.LandscapeClientConfig()
			require.NoError(t, err, "Subscription should not return any errors")
			require.Equal(t, landscapeConf1, lcape, "Subscription did not return the landscape config we wrote")
			require.Equal(t, config.SourceRegistry, src, "Subscription did not come from registry")

			// Enter a second set of data to override the first one
			err = c.UpdateRegistryData(ctx, config.RegistryData{
				UbuntuProToken:  proToken2,
				LandscapeConfig: landscapeConf2,
			}, db)
			require.NoError(t, err, "UpdateRegistryData should not have failed")

			tokenCsum2, lcapeCsum2 := loadChecksums(t, dir)
			require.NotEmpty(t, tokenCsum2, "Subscription checksum should not be empty")
			require.NotEmpty(t, lcapeCsum2, "Landscape checksum should not be empty")
			require.NotEqual(t, tokenCsum1, tokenCsum2, "Subscription checksum should have changed")
			require.NotEqual(t, lcapeCsum1, lcapeCsum2, "Landscape checksum should have changed")

			require.Equal(t, 1, calledUbuntuProNotifier, "UbuntuProNotifier called an unexpected amount of times")
			require.Equal(t, 1, calledLandscapeNotifier, "LandscapeNotifier called an unexpected amount of times")
			calledUbuntuProNotifier = 0
			calledLandscapeNotifier = 0

			token, src, err = c.Subscription()
			require.NoError(t, err, "Subscription should not return any errors")
			require.Equal(t, proToken2, token, "Subscription did not return the token we wrote")
			require.Equal(t, config.SourceRegistry, src, "Subscription did not come from registry")

			lcape, src, err = c.LandscapeClientConfig()
			require.NoError(t, err, "Subscription should not return any errors")
			require.Equal(t, landscapeConf2, lcape, "Subscription did not return the landscape config we wrote")
			require.Equal(t, config.SourceRegistry, src, "Subscription did not come from registry")

			// Enter the second set of data again
			err = c.UpdateRegistryData(ctx, config.RegistryData{
				UbuntuProToken:  proToken2,
				LandscapeConfig: landscapeConf2,
			}, db)
			require.NoError(t, err, "UpdateRegistryData should not have failed")

			tokenCsum3, lcapeCsum3 := loadChecksums(t, dir)
			require.Equal(t, tokenCsum2, tokenCsum3, "Subscription checksum should not have changed")
			require.Equal(t, lcapeCsum2, lcapeCsum3, "Landscape checksum should not have changed")

			require.Zero(t, calledUbuntuProNotifier, "UbuntuProNotifier called an unexpected amount of times")
			require.Zero(t, calledLandscapeNotifier, "LandscapeNotifier called an unexpected amount of times")
			calledUbuntuProNotifier = 0
			calledLandscapeNotifier = 0

			token, src, err = c.Subscription()
			require.NoError(t, err, "Subscription should not return any errors")
			require.Equal(t, proToken2, token, "Subscription did not return the token we wrote")
			require.Equal(t, config.SourceRegistry, src, "Subscription did not come from registry")

			lcape, src, err = c.LandscapeClientConfig()
			require.NoError(t, err, "Subscription should not return any errors")
			require.Equal(t, landscapeConf2, lcape, "Subscription did not return the landscape config we wrote")
			require.Equal(t, config.SourceRegistry, src, "Subscription did not come from registry")

			// Change only the pro token
			err = c.UpdateRegistryData(ctx, config.RegistryData{
				UbuntuProToken:  proToken1,
				LandscapeConfig: landscapeConf2,
			}, db)
			require.NoError(t, err, "UpdateRegistryData should not have failed")

			require.Equal(t, 1, calledUbuntuProNotifier, "UbuntuProNotifier called an unexpected amount of times")
			require.Zero(t, calledLandscapeNotifier, "LandscapeNotifier called an unexpected amount of times")
			calledUbuntuProNotifier = 0
			calledLandscapeNotifier = 0

			// Change only the landscape config
			err = c.UpdateRegistryData(ctx, config.RegistryData{
				UbuntuProToken:  proToken1,
				LandscapeConfig: landscapeConf1,
			}, db)
			require.NoError(t, err, "UpdateRegistryData should not have failed")

			require.Zero(t, calledUbuntuProNotifier, "UbuntuProNotifier called an unexpected amount of times")
			require.Equal(t, 1, calledLandscapeNotifier, "LandscapeNotifier called an unexpected amount of times")
			calledUbuntuProNotifier = 0
			calledLandscapeNotifier = 0
		})
	}
}

// loadChecksums is a test helper that loads the checksums from the config file.
func loadChecksums(t *testing.T, confDir string) (string, string) {
	t.Helper()

	var fileData struct {
		Landscape    struct{ Checksum string }
		Subscription struct{ Checksum string }
	}

	out, err := os.ReadFile(filepath.Join(confDir, "config"))
	require.NoError(t, err, "Could not read config file")

	err = yaml.Unmarshal(out, &fileData)
	require.NoError(t, err, "Could not marshal config file")

	return fileData.Subscription.Checksum, fileData.Landscape.Checksum
}

// is defines equality between flags. It is convenience function to check if a settingsState matches a certain state.
func (state settingsState) is(flag settingsState) bool {
	return state&flag == flag
}

//nolint:revive // testing.T always first!
func setUpMockSettings(t *testing.T, ctx context.Context, db *database.DistroDB, state settingsState, fileBroken, fileCannotWrite bool) (func(*testing.T, *config.Config), string) {
	t.Helper()

	// Sets up the config
	setupConfig := func(t *testing.T, c *config.Config) {
		t.Helper()

		var d config.RegistryData
		var anyData bool

		if state.is(orgTokenHasValue) {
			d.UbuntuProToken = "org_token"
			anyData = true
		}

		if state.is(orgLandscapeConfigHasValue) {
			d.LandscapeConfig = "[client]\nuser=BigOrg"
			anyData = true
		}

		if !anyData {
			return
		}

		err := c.UpdateRegistryData(ctx, d, db)
		require.NoError(t, err, "Setup: could not set config registry data")
	}

	// Mock file config
	cacheDir := t.TempDir()
	var filemode fs.FileMode = 0600
	if fileCannotWrite {
		filemode = 0444 // read-only
	}
	if fileBroken {
		err := os.MkdirAll(filepath.Join(cacheDir, "config"), filemode)
		require.NoError(t, err, "Setup: could not create directory to interfere with config")
		return setupConfig, cacheDir
	}

	if !state.is(fileExists) {
		return setupConfig, cacheDir
	}

	fileData := struct {
		Landscape    map[string]string
		Subscription map[string]string
	}{
		Subscription: make(map[string]string),
		Landscape:    make(map[string]string),
	}

	if state.is(userTokenExists) {
		fileData.Subscription["user"] = ""
	}
	if state.is(userTokenHasValue) {
		fileData.Subscription["user"] = "user_token"
	}

	if state.is(storeTokenExists) {
		fileData.Subscription["store"] = ""
	}
	if state.is(storeTokenHasValue) {
		fileData.Subscription["store"] = "store_token"
	}

	if state.is(userLandscapeConfigExists) {
		fileData.Landscape["config"] = ""
	}
	if state.is(userLandscapeConfigHasValue) {
		fileData.Landscape["config"] = "[client]\nuser=JohnDoe"
	}

	if state.is(landscapeUIDExists) {
		fileData.Landscape["uid"] = ""
	}
	if state.is(landscapeUIDHasValue) {
		fileData.Landscape["uid"] = "landscapeUID1234"
	}

	out, err := yaml.Marshal(fileData)
	require.NoError(t, err, "Setup: could not marshal fake config")

	err = os.WriteFile(filepath.Join(cacheDir, "config"), out, filemode)
	require.NoError(t, err, "Setup: could not write config file")

	return setupConfig, cacheDir
}
