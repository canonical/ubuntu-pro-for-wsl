package config_test

import (
	"context"
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"github.com/canonical/ubuntu-pro-for-wsl/common/testutils"
	config "github.com/canonical/ubuntu-pro-for-wsl/windows-agent/internal/config"
	"github.com/canonical/ubuntu-pro-for-wsl/windows-agent/internal/distros/database"
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
	landscapeIsNotINI           = landscapeUIDExists | 1<<24        // File exists, landscape client config exists, but is not INI syntax
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

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			ctx := context.Background()
			if wsl.MockAvailable() {
				t.Parallel()
				ctx = wsl.WithMock(ctx, wslmock.New())
			}

			db, err := database.New(ctx, t.TempDir())
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

		wantSource config.Source
		wantError  bool
	}{
		"Retrieves existing config user data":                              {settingsState: userLandscapeConfigHasValue, wantSource: config.SourceUser},
		"Retrieves existing config user data containing the hostagent UID": {settingsState: userLandscapeConfigHasValue | landscapeUIDHasValue, wantSource: config.SourceUser},

		"Success when there is no registry and user data": {settingsState: untouched, wantSource: config.SourceNone},

		"Retrieves organization data":                     {settingsState: orgLandscapeConfigHasValue, wantSource: config.SourceRegistry},
		"Retrieves org data containing the hostagent UID": {settingsState: orgLandscapeConfigHasValue | landscapeUIDHasValue, wantSource: config.SourceRegistry},

		"Prioritizes organization config over a user config": {settingsState: orgLandscapeConfigHasValue | userLandscapeConfigHasValue, wantSource: config.SourceRegistry},

		"Error when the file cannot be read": {settingsState: untouched, breakFile: true, wantError: true},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			ctx := context.Background()
			if wsl.MockAvailable() {
				t.Parallel()
				ctx = wsl.WithMock(ctx, wslmock.New())
			}

			db, err := database.New(ctx, t.TempDir())
			require.NoError(t, err, "Setup: could not create empty database")

			setup, dir := setUpMockSettings(t, ctx, db, tc.settingsState, tc.breakFile, false)
			conf := config.New(ctx, dir)
			setup(t, conf)

			landscapeConf, source, err := conf.LandscapeClientConfig()
			if tc.wantError {
				require.Error(t, err, "LandscapeClientConfig should return an error")
				return
			}
			require.NoError(t, err, "LandscapeClientConfig should return no error")

			want := testutils.LoadWithUpdateFromGolden(t, landscapeConf)

			require.Equal(t, want, landscapeConf, "Unexpected Landscape config value")
			require.Equal(t, tc.wantSource, source, "Unexpected Landscape config source")
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
		t.Run(name, func(t *testing.T) {
			ctx := context.Background()
			if wsl.MockAvailable() {
				t.Parallel()
				ctx = wsl.WithMock(ctx, wslmock.New())
			}

			db, err := database.New(ctx, t.TempDir())
			require.NoError(t, err, "Setup: could not create empty database")

			setup, dir := setUpMockSettings(t, ctx, db, tc.settingsState, tc.breakFile, false)
			conf := config.New(ctx, dir)
			setup(t, conf)
			if tc.breakFileContents {
				err := os.WriteFile(filepath.Join(dir, "config"), []byte("\tmessage:\n\t\tthis is not YAML!["), 0600)
				require.NoError(t, err, "Setup: could not re-write config file")
			}

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
		t.Run(name, func(t *testing.T) {
			ctx := context.Background()
			if wsl.MockAvailable() {
				t.Parallel()
				ctx = wsl.WithMock(ctx, wslmock.New())
			}

			db, err := database.New(ctx, t.TempDir())
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
		t.Run(name, func(t *testing.T) {
			ctx := context.Background()
			if wsl.MockAvailable() {
				t.Parallel()
				ctx = wsl.WithMock(ctx, wslmock.New())
			}

			db, err := database.New(ctx, t.TempDir())
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
	if wsl.MockAvailable() {
		t.Parallel()
	}

	const landscapeBaseConf = "[host]\nurl=127.0.0.1:8080\n[client]\nuser=JohnDoe"
	testCases := map[string]struct {
		settingsState   settingsState
		breakFile       bool
		landscapeConfig string

		wantError bool
	}{
		"Saves the config when there was no previous data":       {settingsState: untouched},
		"Accepts IPv6 in the [host].url key":                     {settingsState: untouched, landscapeConfig: "[host]\nurl=[2001:db8::1]:6554\n[client]\nsomething=else"},
		"Merges user-submitted data with existing hostagent UID": {settingsState: userLandscapeConfigExists | landscapeUIDHasValue},
		"Merges user-submitted discarding new hostagent UID":     {settingsState: userLandscapeConfigExists | landscapeUIDHasValue, landscapeConfig: landscapeBaseConf + "\nhostagent_uid=new_and_discarded_hostagent_uid\n"},
		"Saves empty new user config data":                       {settingsState: userLandscapeConfigHasValue, landscapeConfig: "-"},

		"Error when the configuration sent is not valid ini syntax":      {settingsState: untouched, landscapeConfig: "NOT INI SYNTAX", wantError: true},
		"Error when the configuration does not contain [client] section": {settingsState: untouched, landscapeConfig: "[host]\nurl=127.0.0.1:8080", wantError: true},
		"Error when the configuration does not contain [host] section":   {settingsState: untouched, landscapeConfig: "[client]\nsomething=else", wantError: true},
		"Error when the configuration does not contain [host].url key":   {settingsState: untouched, landscapeConfig: "[host]\nvalue=127.0.0.1:8080\n[client]\nsomething=else", wantError: true},
		"Error when the [host].url has scheme":                           {settingsState: untouched, landscapeConfig: "[host]\nurl=http://127.0.0.1:8080\n[client]\nsomething=else", wantError: true},
		"Error when the [host].url host is missing":                      {settingsState: untouched, landscapeConfig: "[host]\nurl=:8080\n[client]\nsomething=else", wantError: true},
		"Error when the [host].url port is missing":                      {settingsState: untouched, landscapeConfig: "[host]\nurl=127.0.0.1\n[client]\nsomething=else", wantError: true},
		"Error when the [host].url port value is invalid":                {settingsState: untouched, landscapeConfig: "[host]\nurl=127.0.0.1:-35\n[client]\nsomething=else", wantError: true},
		"Error when the [host].url port is 0":                            {settingsState: untouched, landscapeConfig: "[host]\nurl=127.0.0.1:0\n[client]\nsomething=else", wantError: true},
		"Error when an organization landscape config is already set":     {settingsState: orgLandscapeConfigHasValue, wantError: true},
		"Error when the config file cannot be read":                      {settingsState: untouched, breakFile: true, wantError: true},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			ctx := context.Background()
			if wsl.MockAvailable() {
				t.Parallel()
				ctx = wsl.WithMock(ctx, wslmock.New())
			}

			db, err := database.New(ctx, t.TempDir())
			require.NoError(t, err, "Setup: could not create empty database")

			setup, dir := setUpMockSettings(t, ctx, db, tc.settingsState, tc.breakFile, false)
			conf := config.New(ctx, dir)
			setup(t, conf)

			wantSource := config.SourceUser
			switch tc.landscapeConfig {
			case "":
				tc.landscapeConfig = landscapeBaseConf
			case "-":
				tc.landscapeConfig = ""
				wantSource = config.SourceNone
			default:
			}

			var calledLandscapeNotifier int
			conf.SetUbuntuProNotifier(func(context.Context, string) {
				require.Fail(t, "UbuntuPro should not be called")
			})

			conf.SetLandscapeNotifier(func(context.Context, string, string) {
				calledLandscapeNotifier++
			})

			err = conf.SetUserLandscapeConfig(ctx, tc.landscapeConfig)
			if tc.wantError {
				require.Error(t, err, "SetUserLandscapeConfig should return an error")
				return
			}
			require.NoError(t, err, "SetUserLandscapeConfig should return no errors")

			got, src, err := conf.LandscapeClientConfig()
			require.NoError(t, err, "LandscapeClientConfig should return no errors")
			require.Equal(t, wantSource, src, "Did not get the same source for Landscape config as we set")
			require.Equal(t, 1, calledLandscapeNotifier, "LandscapeNotifier should have been called once")

			if wantSource == config.SourceNone {
				require.Empty(t, got, "Did not get the same value for Landscape config as we set")
				return
			}

			want := testutils.LoadWithUpdateFromGolden(t, got)

			require.Equal(t, want, got, "Did not get the same value for Landscape config as we set")
		})
	}
}

func TestSetLandscapeAgentUID(t *testing.T) {
	if wsl.MockAvailable() {
		t.Parallel()
	}

	testCases := map[string]struct {
		settingsState   settingsState
		uid             string
		breakFile       bool
		cannotWriteFile bool

		want       string
		wantNotify bool
		wantError  bool
	}{
		"Sets the UID with previous client conf":     {settingsState: userLandscapeConfigHasValue, want: "new_uid", wantNotify: true},
		"Sets the UID with the same value":           {settingsState: landscapeUIDHasValue, uid: "landscapeUID1234", want: "landscapeUID1234"},
		"Sets the UID with previous org client conf": {settingsState: orgLandscapeConfigHasValue, want: "new_uid", wantNotify: true},

		"Overrides the UID with previous client conf": {settingsState: userLandscapeConfigHasValue | landscapeUIDHasValue, want: "new_uid", wantNotify: true},

		"Resets the UID with previous client conf":     {settingsState: userLandscapeConfigHasValue | landscapeUIDHasValue, uid: "-", want: "", wantNotify: true},
		"Resets the UID with previous org client conf": {settingsState: orgLandscapeConfigHasValue | landscapeUIDHasValue, uid: "-", want: "", wantNotify: true},

		"Cannot set the UID without previous client conf":      {settingsState: untouched, want: ""},
		"Cannot reset the UID with no previous client conf":    {settingsState: landscapeUIDHasValue, uid: "-", want: "landscapeUID1234"},
		"Cannot override the UID with no previous client conf": {settingsState: untouched, want: ""},

		"Error when the file cannot be read":             {settingsState: untouched, breakFile: true, wantError: true},
		"Error when the previous client conf is invalid": {settingsState: userLandscapeConfigHasValue | landscapeIsNotINI, wantError: true},
		"Error when the file cannot be written":          {settingsState: userLandscapeConfigHasValue, cannotWriteFile: true, want: "", wantError: true},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			ctx := context.Background()
			if wsl.MockAvailable() {
				t.Parallel()
				ctx = wsl.WithMock(ctx, wslmock.New())
			}

			db, err := database.New(ctx, t.TempDir())
			require.NoError(t, err, "Setup: could not create empty database")

			setup, dir := setUpMockSettings(t, ctx, db, tc.settingsState, tc.breakFile, tc.cannotWriteFile)
			conf := config.New(ctx, dir)
			setup(t, conf)

			switch tc.uid {
			case "":
				tc.uid = "new_uid"
			case "-":
				tc.uid = ""
			default:
			}

			conf.SetUbuntuProNotifier(func(context.Context, string) {
				require.Fail(t, "UbuntuProNotifier should not be called")
			})

			conf.SetLandscapeNotifier(func(context.Context, string, string) {
				if !tc.wantNotify {
					require.Fail(t, "LandscapeNotifier should not have been called")
				}
			})

			err = conf.SetLandscapeAgentUID(ctx, tc.uid)
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
		landscapeConf1 = "[host]\nurl=127.0.0.1:8080\n[client]\ngreeting=hello"

		proToken2      = "UBUNTU_PRO_TOKEN_SECOND"
		landscapeConf2 = "[host]\nurl=127.0.0.1:8080\n[client]\ngreeting=cheers"

		invalidLandscapeConf = "NOT AN INI SYNTAX"
	)

	testCases := map[string]struct {
		settingsState   settingsState
		breakConfigFile bool

		wantErr bool
	}{
		"Success":                             {},
		"Registry data overrides user config": {settingsState: userTokenHasValue | userLandscapeConfigHasValue},

		"Error when we cannot load from file": {breakConfigFile: true, wantErr: true},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			ctx := context.Background()
			if wsl.MockAvailable() {
				t.Parallel()
				ctx = wsl.WithMock(ctx, wslmock.New())
			}

			db, err := database.New(ctx, t.TempDir())
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
			require.NoError(t, err, "LandscapeClientConfig should not return any errors")

			basepath := testutils.TestFixturePath(t)
			want := testutils.LoadWithUpdateFromGolden(t, lcape, testutils.WithGoldenPath(filepath.Join(basepath, "step1_override_defaults")))
			require.Equal(t, want, lcape, "LandscapeClientConfig did not return the Landscape config we wrote")
			require.Equal(t, config.SourceRegistry, src, "Landscape config did not come from registry")

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
			require.NoError(t, err, "LandscapeClientConfig should not return any errors")

			want = testutils.LoadWithUpdateFromGolden(t, lcape, testutils.WithGoldenPath(filepath.Join(basepath, "step2_override_previous")))
			require.Equal(t, want, lcape, "LandscapeClientConfig did not return the Landscape config we wrote")
			require.Equal(t, config.SourceRegistry, src, "Landscape config did not come from registry")

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
			require.NoError(t, err, "LandscapeClientConfig should not return any errors")
			want = testutils.LoadWithUpdateFromGolden(t, lcape, testutils.WithGoldenPath(filepath.Join(basepath, "step3_no_change")))
			require.Equal(t, want, lcape, "LandscapeClientConfig did not return the landscape config we wrote")
			require.Equal(t, config.SourceRegistry, src, "Landscape config did not come from registry")

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

			// Apply invalid Landscape config - should erase the previous one
			err = c.UpdateRegistryData(ctx, config.RegistryData{
				UbuntuProToken:  proToken1,
				LandscapeConfig: invalidLandscapeConf,
			}, db)
			require.NoError(t, err, "UpdateRegistryData should not have failed")
			require.Zero(t, calledUbuntuProNotifier, "UbuntuProNotifier called an unexpected amount of times")
			require.Equal(t, 1, calledLandscapeNotifier, "LandscapeNotifier called an unexpected amount of times")

			lcape, src, err = c.LandscapeClientConfig()
			require.NoError(t, err, "LandscapeClientConfig should not return any errors")

			if tc.settingsState.is(userLandscapeConfigHasValue) {
				require.Equal(t, config.SourceUser, src, "Landscape config should come from user")
			} else {
				require.Empty(t, lcape, "LandscapeClientConfig should return empty string")
				require.Equal(t, config.SourceNone, src, "Landscape config should not exist")
			}
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
			d.LandscapeConfig = "[host]\nurl=landscape.bigorg.com:6554\n[client]\nuser=BigOrg"
			anyData = true
		}

		if anyData {
			require.NoError(t, c.UpdateRegistryData(ctx, d, db), "Setup: could not set config registry data")
		}

		if !fileBroken {
			// Forcing c to load the config file.
			_, err := c.LandscapeAgentUID()
			require.NoError(t, err, "Setup: could not load data from mock config file")
		}
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
		fileData.Landscape["config"] = "[host]\nurl=landscape.canonical.com:6554\n[client]\nuser=JohnDoe"
		if state.is(landscapeIsNotINI) {
			fileData.Landscape["config"] = "NOT INI SYNTAX"
		}
	}

	if state.is(landscapeUIDExists) {
		fileData.Landscape["uid"] = ""
	}
	if state.is(landscapeUIDHasValue) {
		fileData.Landscape["uid"] = "landscapeUID1234"
		if state.is(userLandscapeConfigHasValue) {
			fileData.Landscape["config"] += "\nhostagent_uid=landscapeUID1234\n"
		}
	}

	out, err := yaml.Marshal(fileData)
	require.NoError(t, err, "Setup: could not marshal fake config")

	err = os.WriteFile(filepath.Join(cacheDir, "config"), out, filemode)
	require.NoError(t, err, "Setup: could not write config file")

	return setupConfig, cacheDir
}
