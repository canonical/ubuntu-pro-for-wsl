package config_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

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

func TestStop(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	conf := config.New(ctx, t.TempDir())

	// Trigger a notification that blocks until we desire
	started := make(chan struct{})
	blocker := make(chan struct{})
	conf.Notify(func() {
		close(started)
		<-blocker
	})

	err := conf.SetUserSubscription("HELLO_I_AM_TOKEN")
	require.NoError(t, err, "Setup: SetUserSubscription should return no error")

	const timeout = 10 * time.Second
	select {
	case <-time.After(timeout):
		require.Fail(t, "Setup: config should have notified its observer")
	case <-started:
	}

	// Check that Stop does not return when the observer is running
	returned := make(chan struct{})
	go func() {
		conf.Stop()
		close(returned)
	}()

	select {
	case <-returned:
		require.Fail(t, "Config's Stop should not return until all callbacks are done")
	case <-time.After(timeout):
	}

	// Stop blocking and check that Stop returns
	close(blocker)

	select {
	case <-returned:
	case <-time.After(timeout):
		require.Fail(t, "Config's Stop should have returned after all callbacks were finished")
	}

	// Call Stop again to see that it no longer blocks
	returned = make(chan struct{})
	go func() {
		conf.Stop()
		close(returned)
	}()

	select {
	case <-returned:
	case <-time.After(timeout):
		require.Fail(t, "Config's Stop should not block when called for a second time")
	}
}

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

			setup, dir := setUpMockSettings(t, ctx, db, tc.settingsState, tc.breakFile)
			conf := config.New(ctx, dir)
			defer conf.Stop()
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

			conf.Stop()

			_, _, err = conf.Subscription()
			require.Error(t, err, "Subscription should return error after the config has stopped")
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

			setup, dir := setUpMockSettings(t, ctx, db, tc.settingsState, tc.breakFile)
			conf := config.New(ctx, dir)
			defer conf.Stop()
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

			conf.Stop()

			_, _, err = conf.LandscapeClientConfig()
			require.Error(t, err, "LandscapeClientConfig should return error after the config has stopped")
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

			setup, dir := setUpMockSettings(t, ctx, db, tc.settingsState, tc.breakFile)
			if tc.breakFileContents {
				err := os.WriteFile(filepath.Join(dir, "config"), []byte("\tmessage:\n\t\tthis is not YAML!["), 0600)
				require.NoError(t, err, "Setup: could not re-write config file")
			}

			conf := config.New(ctx, dir)
			defer conf.Stop()
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

			conf.Stop()

			_, err = conf.LandscapeAgentUID()
			require.Error(t, err, "LandscapeAgentUID should return error after the config has stopped")
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

			setup, dir := setUpMockSettings(t, ctx, db, tc.settingsState, false)
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

			conf.Stop()

			_, err = conf.ProvisioningTasks(ctx, "UBUNTU")
			require.Error(t, err, "ProvisioningTasks should return error after the config has stopped")
		})
	}
}

func TestSetUserSubscription(t *testing.T) {
	if wsl.MockAvailable() {
		t.Parallel()
	}

	testCases := map[string]struct {
		settingsState settingsState
		breakFile     bool
		emptyToken    bool

		want      string
		wantError bool
	}{
		"Success":                          {settingsState: userTokenHasValue, want: "new_token"},
		"Success disabling a subscription": {settingsState: userTokenHasValue, emptyToken: true, want: ""},

		"Error when there is a store token active": {settingsState: storeTokenHasValue, wantError: true},
		"Error when the file cannot be opened":     {settingsState: fileExists, breakFile: true, wantError: true},
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

			setup, dir := setUpMockSettings(t, ctx, db, tc.settingsState, tc.breakFile)
			conf := config.New(ctx, dir)
			defer conf.Stop()
			setup(t, conf)

			token := "new_token"
			if tc.emptyToken {
				token = ""
			}

			err = conf.SetUserSubscription(token)
			if tc.wantError {
				require.Error(t, err, "SetSubscription should return an error")
				return
			}
			require.NoError(t, err, "SetSubscription should return no error")

			got, _, err := conf.Subscription()
			require.NoError(t, err, "ProToken should return no error")

			require.Equal(t, tc.want, got, "ProToken returned an unexpected value for the token")

			conf.Stop()

			err = conf.SetUserSubscription(token)
			require.Error(t, err, "SetUserSubscription should return error after the config has stopped")
		})
	}
}

func TestSetStoreSubscription(t *testing.T) {
	if wsl.MockAvailable() {
		t.Parallel()
	}

	testCases := map[string]struct {
		settingsState settingsState
		breakFile     bool
		emptyToken    bool

		want      string
		wantError bool
	}{
		"Success":                          {settingsState: userTokenHasValue, want: "new_token"},
		"Success disabling a subscription": {settingsState: storeTokenHasValue, emptyToken: true, want: ""},
		"Success overriding an existing store token": {settingsState: storeTokenHasValue, want: "new_token"},

		"Error when the file cannot be opened": {settingsState: fileExists, breakFile: true, wantError: true},
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

			setup, dir := setUpMockSettings(t, ctx, db, tc.settingsState, tc.breakFile)
			conf := config.New(ctx, dir)
			setup(t, conf)

			token := "new_token"
			if tc.emptyToken {
				token = ""
			}

			err = conf.SetStoreSubscription(ctx, token)
			if tc.wantError {
				require.Error(t, err, "SetSubscription should return an error")
				return
			}
			require.NoError(t, err, "SetSubscription should return no error")

			got, _, err := conf.Subscription()
			require.NoError(t, err, "ProToken should return no error")

			require.Equal(t, tc.want, got, "ProToken returned an unexpected value for the token")
		})
	}
}

func TestSetLandscapeAgentUID(t *testing.T) {
	if wsl.MockAvailable() {
		t.Parallel()
	}

	testCases := map[string]struct {
		settingsState settingsState
		emptyUID      bool
		breakFile     bool

		want      string
		wantError bool
	}{
		"Success overriding the UID":                      {settingsState: landscapeUIDHasValue, want: "new_uid"},
		"Success unsetting the UID":                       {settingsState: landscapeUIDHasValue, emptyUID: true, want: ""},
		"Success when the file does not exist":            {settingsState: untouched, want: "new_uid"},
		"Success when the pro token field does not exist": {settingsState: fileExists, want: "new_uid"},

		"Error when the file cannot be opened": {settingsState: landscapeUIDHasValue, breakFile: true, want: "landscapeUID1234", wantError: true},
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

			setup, dir := setUpMockSettings(t, ctx, db, tc.settingsState, tc.breakFile)
			conf := config.New(ctx, dir)
			defer conf.Stop()
			setup(t, conf)

			uid := "new_uid"
			if tc.emptyUID {
				uid = ""
			}

			err = conf.SetLandscapeAgentUID(uid)
			if tc.wantError {
				require.Error(t, err, "SetLandscapeAgentUID should return an error")
				return
			}
			require.NoError(t, err, "SetLandscapeAgentUID should return no error")

			got, err := conf.LandscapeAgentUID()
			require.NoError(t, err, "LandscapeAgentUID should return no error")

			require.Equal(t, tc.want, got, "LandscapeAgentUID returned an unexpected value for the token")

			conf.Stop()

			err = conf.SetLandscapeAgentUID(uid)
			require.Error(t, err, "SetLandscapeAgentUID should return error after the config has stopped")
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

			_, dir := setUpMockSettings(t, ctx, db, tc.settingsState, tc.breakConfigFile)
			c := config.New(ctx, dir)
			defer c.Stop()

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

			token, src, err = c.Subscription()
			require.NoError(t, err, "Subscription should not return any errors")
			require.Equal(t, proToken2, token, "Subscription did not return the token we wrote")
			require.Equal(t, config.SourceRegistry, src, "Subscription did not come from registry")

			lcape, src, err = c.LandscapeClientConfig()
			require.NoError(t, err, "Subscription should not return any errors")
			require.Equal(t, landscapeConf2, lcape, "Subscription did not return the landscape config we wrote")
			require.Equal(t, config.SourceRegistry, src, "Subscription did not come from registry")

			c.Stop()

			err = c.UpdateRegistryData(ctx, config.RegistryData{}, db)
			require.Error(t, err, "UpdateRegistryData should return error after the config has stopped")
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

func TestNotify(t *testing.T) {
	ctx := context.Background()
	if wsl.MockAvailable() {
		t.Parallel()
		ctx = wsl.WithMock(ctx, wslmock.New())
	}

	db, err := database.New(ctx, t.TempDir(), nil)
	require.NoError(t, err, "Setup: could not create empty database")

	_, dir := setUpMockSettings(t, ctx, db, untouched, false)
	c := config.New(ctx, dir)
	defer c.Stop()

	var notifyCount atomic.Int32
	var wantNotifyCount int32

	c.Notify(func() { notifyCount.Add(1) })

	err = c.SetUserSubscription("TOKEN_1")
	require.NoError(t, err, "SetUserSubscription should return no error")
	wantNotifyCount++

	eventually(t, notifyCount.Load, func(got int32) bool { return wantNotifyCount == got },
		time.Second, 100*time.Millisecond, "Attached function should have been called after changing the pro token")

	err = c.SetLandscapeAgentUID("UID_1")
	require.NoError(t, err, "SetLandscapeAgentUID should return no error")
	wantNotifyCount++

	eventually(t, notifyCount.Load, func(got int32) bool { return wantNotifyCount == got },
		time.Second, 100*time.Millisecond, "Attached function should have been called after changing the landscape UID")

	err = c.UpdateRegistryData(ctx, config.RegistryData{UbuntuProToken: "TOKEN_2"}, db)
	require.NoError(t, err, "UpdateRegistryData should return no error")
	wantNotifyCount++

	eventually(t, notifyCount.Load, func(got int32) bool { return wantNotifyCount == got },
		time.Second, 100*time.Millisecond, "Attached function should have been called after changing registry data")
}

// eventually solves the main issue with 'require.Eventually': you don't know what failed.
// See what happens if you try to build the 'want vs. got' error message:
//
//	  require.Eventuallyf(t, func()bool{ return 5==got() },
//			t, trate, "Mismatch: wanted %d but got %d", 5, got())
//	                                                       ^^^
//														   OUTDATED!
//
// Got is passed by value so we have the value obtained at t=0, not at t=timeout.
//
//nolint:thelper // This is not a helper but an assertion itself.
func eventually[T any](t *testing.T, getter func() T, predicate func(T) bool, timeout, tickRate time.Duration, message string, args ...any) {
	tk := time.NewTicker(tickRate)
	defer tk.Stop()

	tm := time.NewTimer(timeout)
	defer tk.Stop()

	got := getter()
	if predicate(got) {
		return
	}

	for {
		select {
		case <-tm.C:
			require.Failf(t, "Condition not satisfied", "Last value: %v\n%s", got, fmt.Sprintf(message, args...))
		case <-tk.C:
		}

		got = getter()
		if predicate(got) {
			return
		}
	}
}

// is defines equality between flags. It is convenience function to check if a settingsState matches a certain state.
func (state settingsState) is(flag settingsState) bool {
	return state&flag == flag
}

//nolint:revive // testing.T always first!
func setUpMockSettings(t *testing.T, ctx context.Context, db *database.DistroDB, state settingsState, fileBroken bool) (func(*testing.T, *config.Config), string) {
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
	if fileBroken {
		err := os.MkdirAll(filepath.Join(cacheDir, "config"), 0600)
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

	err = os.WriteFile(filepath.Join(cacheDir, "config"), out, 0600)
	require.NoError(t, err, "Setup: could not write config file")

	return setupConfig, cacheDir
}
