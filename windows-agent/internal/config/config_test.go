package config_test

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"github.com/canonical/ubuntu-pro-for-windows/common/wsltestutils"
	config "github.com/canonical/ubuntu-pro-for-windows/windows-agent/internal/config"
	"github.com/canonical/ubuntu-pro-for-windows/windows-agent/internal/config/registry"
	"github.com/canonical/ubuntu-pro-for-windows/windows-agent/internal/distros/database"
	"github.com/canonical/ubuntu-pro-for-windows/windows-agent/internal/distros/distro"
	"github.com/canonical/ubuntu-pro-for-windows/windows-agent/internal/distros/task"
	"github.com/canonical/ubuntu-pro-for-windows/windows-agent/internal/tasks"
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
	keyExists  settingsState = 1 // Key exists but is empty
	fileExists settingsState = 2 // File exists but is empty

	// Registry settings.
	orgTokenExists           = keyExists | 1<<3 // Key exists, organization token exists
	orgLandscapeConfigExists = keyExists | 1<<4 // Key exists, organization landscape config exists

	orgTokenHasValue           = orgTokenExists | 1<<5 // Key exists, organization token exists, and is not empty
	orgLandscapeConfigHasValue = orgTokenExists | 1<<6 // Key exists, organization landscape config , and is not empty

	// File settings.
	userTokenExists           = fileExists | 1<<(7+iota) // File exists, user token exists
	storeTokenExists                                     // File exists, microsoft store token exists
	userLandscapeConfigExists                            // File exists, landscape client config exists
	landscapeUIDExists                                   // File exists, landscape agent UID exists

	userTokenHasValue           = userTokenExists | 1<<20           // File exists, user token exists, and is not empty
	storeTokenHasValue          = storeTokenExists | 1<<21          // File exists, microsoft store token exists, and is not empty
	userLandscapeConfigHasValue = userLandscapeConfigExists | 1<<22 // File exists, landscape client config exists, and is not empty
	landscapeUIDHasValue        = landscapeUIDExists | 1<<23        // File exists, landscape agent UID exists, and is not empty
)

func TestSubscription(t *testing.T) {
	t.Parallel()

	testCases := map[string]struct {
		mockErrors    uint32
		breakFile     bool
		settingsState settingsState

		wantToken  string
		wantSource config.Source
		wantError  bool
	}{
		"Success": {settingsState: userTokenHasValue, wantToken: "user_token", wantSource: config.SourceUser},
		"Success when neither registry key nor conf file exist": {settingsState: untouched},
		"Success when the key exists but is empty":              {settingsState: keyExists},
		"Success when the key exists but contains empty fields": {settingsState: orgTokenExists},

		"Success when there is an organization token": {settingsState: orgTokenHasValue, wantToken: "org_token", wantSource: config.SourceRegistry},
		"Success when there is a user token":          {settingsState: userTokenHasValue, wantToken: "user_token", wantSource: config.SourceUser},
		"Success when there is a store token":         {settingsState: storeTokenHasValue, wantToken: "store_token", wantSource: config.SourceMicrosoftStore},

		"Success when there are organization and user tokens":                           {settingsState: orgTokenHasValue | userTokenHasValue, wantToken: "user_token", wantSource: config.SourceUser},
		"Success when there are organization and store tokens":                          {settingsState: orgTokenHasValue | storeTokenHasValue, wantToken: "store_token", wantSource: config.SourceMicrosoftStore},
		"Success when there are organization and user tokens, and an empty store token": {settingsState: orgTokenHasValue | userTokenHasValue | storeTokenExists, wantToken: "user_token", wantSource: config.SourceUser},

		"Error when the registry key cannot be opened":    {settingsState: orgTokenHasValue, mockErrors: registry.MockErrOnOpenKey, wantError: true},
		"Error when the registry key cannot be read from": {settingsState: orgTokenHasValue, mockErrors: registry.MockErrReadValue, wantError: true},
		"Error when the file cannot be read from":         {settingsState: untouched, breakFile: true, wantError: true},
	}

	for name, tc := range testCases {
		tc := tc
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			ctx := context.Background()

			r, dir := setUpMockSettings(t, tc.mockErrors, tc.settingsState, false, tc.breakFile)
			conf := config.New(ctx, dir, config.WithRegistry(r))

			token, source, err := conf.Subscription(ctx)
			if tc.wantError {
				require.Error(t, err, "ProToken should return an error")
				return
			}
			require.NoError(t, err, "ProToken should return no error")

			// Test values
			require.Equal(t, tc.wantToken, token, "Unexpected token value")
			require.Equal(t, tc.wantSource, source, "Unexpected token source")
			assert.Zero(t, r.OpenKeyCount.Load(), "Leaking keys after ProToken")
		})
	}
}

func TestLandscapeConfig(t *testing.T) {
	t.Parallel()

	testCases := map[string]struct {
		mockErrors    uint32
		breakFile     bool
		settingsState settingsState

		wantLandscapeConfig string
		wantSource          config.Source
		wantError           bool
	}{
		"Success": {settingsState: userLandscapeConfigHasValue, wantLandscapeConfig: "[client]\nuser=JohnDoe", wantSource: config.SourceUser},

		"Success when neither registry key nor conf file exist":          {settingsState: untouched},
		"Success when the registry key exists but is empty":              {settingsState: keyExists},
		"Success when the registry key exists but contains empty fields": {settingsState: orgLandscapeConfigExists},

		"Success when there is an organization conf": {settingsState: orgLandscapeConfigHasValue, wantLandscapeConfig: "[client]\nuser=BigOrg", wantSource: config.SourceRegistry},
		"Success when there is a user conf":          {settingsState: userLandscapeConfigHasValue, wantLandscapeConfig: "[client]\nuser=JohnDoe", wantSource: config.SourceUser},

		"Success when there are organization and user confs": {settingsState: orgLandscapeConfigHasValue | userLandscapeConfigHasValue, wantLandscapeConfig: "[client]\nuser=JohnDoe", wantSource: config.SourceUser},

		"Error when the registry key cannot be opened":    {settingsState: orgTokenHasValue, mockErrors: registry.MockErrOnOpenKey, wantError: true},
		"Error when the registry key cannot be read from": {settingsState: orgTokenHasValue, mockErrors: registry.MockErrReadValue, wantError: true},
		"Error when the file cannot be read from":         {settingsState: untouched, breakFile: true, wantError: true},
	}

	for name, tc := range testCases {
		tc := tc
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			ctx := context.Background()

			r, dir := setUpMockSettings(t, tc.mockErrors, tc.settingsState, false, tc.breakFile)
			conf := config.New(ctx, dir, config.WithRegistry(r))

			token, source, err := conf.LandscapeClientConfig(ctx)
			if tc.wantError {
				require.Error(t, err, "ProToken should return an error")
				return
			}
			require.NoError(t, err, "ProToken should return no error")

			// Test values
			require.Equal(t, tc.wantLandscapeConfig, token, "Unexpected token value")
			require.Equal(t, tc.wantSource, source, "Unexpected token source")
			assert.Zero(t, r.OpenKeyCount.Load(), "Leaking keys after ProToken")
		})
	}
}

func TestLandscapeAgentUID(t *testing.T) {
	t.Parallel()

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
			t.Parallel()
			ctx := context.Background()

			r, dir := setUpMockSettings(t, 0, tc.settingsState, false, tc.breakFile)
			if tc.breakFileContents {
				err := os.WriteFile(filepath.Join(dir, "config"), []byte("\tmessage:\n\t\tthis is not YAML!["), 0600)
				require.NoError(t, err, "Setup: could not re-write config file")
			}
			conf := config.New(ctx, dir, config.WithRegistry(r))

			v, err := conf.LandscapeAgentUID(ctx)
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
			assert.Zero(t, r.OpenKeyCount.Load(), "Call to LandscapeAgentUID leaks registry keys")
		})
	}
}

func TestProvisioningTasks(t *testing.T) {
	t.Parallel()

	testCases := map[string]struct {
		mockErrors    uint32
		settingsState settingsState

		wantToken         string
		wantLandscapeConf string
		wantLandscapeUID  string

		wantNoLandscape bool
		wantError       bool
	}{
		"Success when the key does not exist":                {settingsState: untouched},
		"Success when the pro token field does not exist":    {settingsState: fileExists},
		"Success when the pro token exists but is empty":     {settingsState: userTokenExists},
		"Success with a user token":                          {settingsState: userTokenHasValue, wantToken: "user_token"},
		"Success when there is Landscape config, but no UID": {settingsState: userLandscapeConfigHasValue, wantNoLandscape: true},
		"Success when there is Landscape config and UID":     {settingsState: userLandscapeConfigHasValue | landscapeUIDHasValue, wantLandscapeConf: "[client]\nuser=JohnDoe", wantLandscapeUID: "landscapeUID1234"},

		"Error when the registry key cannot be opened":    {settingsState: orgTokenExists, mockErrors: registry.MockErrOnOpenKey, wantError: true},
		"Error when the registry key cannot be read from": {settingsState: orgTokenExists, mockErrors: registry.MockErrReadValue, wantError: true},
	}

	for name, tc := range testCases {
		tc := tc
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			ctx := context.Background()

			r, dir := setUpMockSettings(t, tc.mockErrors, tc.settingsState, false, false)
			conf := config.New(ctx, dir, config.WithRegistry(r))

			gotTasks, err := conf.ProvisioningTasks(ctx, "UBUNTU")
			if tc.wantError {
				require.Error(t, err, "ProvisioningTasks should return an error")
				return
			}
			require.NoError(t, err, "ProvisioningTasks should return no error")

			wantTasks := []task.Task{
				tasks.ProAttachment{Token: tc.wantToken},
			}

			if !tc.wantNoLandscape {
				wantTasks = append(wantTasks, tasks.LandscapeConfigure{
					Config:       tc.wantLandscapeConf,
					HostagentUID: tc.wantLandscapeUID,
				})
			}

			require.ElementsMatch(t, wantTasks, gotTasks, "Unexpected contents returned by ProvisioningTasks")
		})
	}
}

func TestSetUserSubscription(t *testing.T) {
	t.Parallel()

	testCases := map[string]struct {
		settingsState settingsState
		breakFile     bool
		emptyToken    bool

		want      string
		wantError bool
	}{
		"Success":                                         {settingsState: userTokenHasValue, want: "new_token"},
		"Success disabling a subscription":                {settingsState: userTokenHasValue, emptyToken: true, want: ""},
		"Success when the key does not exist":             {settingsState: untouched, want: "new_token"},
		"Success when the pro token field does not exist": {settingsState: keyExists, want: "new_token"},
		"Success when there is a store token active":      {settingsState: storeTokenHasValue, want: "store_token"},

		"Error when the file cannot be opened": {settingsState: fileExists, breakFile: true, wantError: true},
	}

	for name, tc := range testCases {
		tc := tc
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			ctx := context.Background()

			r, dir := setUpMockSettings(t, 0, tc.settingsState, false, tc.breakFile)
			conf := config.New(ctx, dir, config.WithRegistry(r))

			token := "new_token"
			if tc.emptyToken {
				token = ""
			}

			err := conf.SetUserSubscription(ctx, token)
			if tc.wantError {
				require.Error(t, err, "SetSubscription should return an error")
				return
			}
			require.NoError(t, err, "SetSubscription should return no error")

			// Disable errors so we can retrieve the token
			r.Errors = 0
			got, _, err := conf.Subscription(ctx)
			require.NoError(t, err, "ProToken should return no error")

			require.Equal(t, tc.want, got, "ProToken returned an unexpected value for the token")
		})
	}
}

func TestSetLandscapeAgentUID(t *testing.T) {
	t.Parallel()

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
			t.Parallel()
			ctx := context.Background()

			r, dir := setUpMockSettings(t, 0, tc.settingsState, false, tc.breakFile)
			conf := config.New(ctx, dir, config.WithRegistry(r))

			uid := "new_uid"
			if tc.emptyUID {
				uid = ""
			}

			err := conf.SetLandscapeAgentUID(ctx, uid)
			if tc.wantError {
				require.Error(t, err, "SetLandscapeAgentUID should return an error")
				return
			}
			require.NoError(t, err, "SetLandscapeAgentUID should return no error")

			// Disable errors so we can retrieve the UID
			r.Errors = 0
			got, err := conf.LandscapeAgentUID(ctx)
			require.NoError(t, err, "LandscapeAgentUID should return no error")

			require.Equal(t, tc.want, got, "LandscapeAgentUID returned an unexpected value for the token")
		})
	}
}

func TestIsReadOnly(t *testing.T) {
	t.Parallel()

	testCases := map[string]struct {
		settingsState settingsState
		readOnly      bool
		registryErr   bool

		want    bool
		wantErr bool
	}{
		"Success when the registry can be written on":    {settingsState: keyExists, want: false},
		"Success when the registry cannot be written on": {settingsState: keyExists, readOnly: true, want: true},

		"Success when the non-existent registry can be written on":    {want: false},
		"Success when the non-existent registry cannot be written on": {readOnly: true, want: true},

		"Error when the registry cannot be queried": {registryErr: true, wantErr: true},
	}

	for name, tc := range testCases {
		tc := tc
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			ctx := context.Background()

			r, dir := setUpMockSettings(t, 0, tc.settingsState, tc.readOnly, false)
			if tc.registryErr {
				r.Errors = registry.MockErrOnCreateKey
			}

			conf := config.New(ctx, dir, config.WithRegistry(r))

			got, err := conf.IsReadOnly()
			if tc.wantErr {
				require.Error(t, err, "IsReadOnly should return an error")
				return
			}
			require.NoError(t, err, "IsReadOnly should return no error")

			require.Equal(t, tc.want, got, "IsReadOnly returned an unexpected value")
		})
	}
}

func TestFetchMicrosoftStoreSubscription(t *testing.T) {
	t.Parallel()

	testCases := map[string]struct {
		settingsState      settingsState
		registryErr        uint32
		registryIsReadOnly bool

		wantToken string
		wantErr   bool
	}{
		// TODO: Implement more test cases when the MS Store mock is available. There is no single successful test in here so far.
		"Error when registry is read only":          {settingsState: userTokenHasValue, registryIsReadOnly: true, wantToken: "user_token", wantErr: true},
		"Error when registry read-only check fails": {registryErr: registry.MockErrOnCreateKey, wantErr: true},

		// Stub test-case: Must be replaced with Success/Error return values of contracts.ProToken
		// when the Microsoft store dance is implemented.
		"Error when the microsoft store is not yet implemented": {wantErr: true},
	}

	for name, tc := range testCases {
		tc := tc
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			ctx := context.Background()

			r, dir := setUpMockSettings(t, tc.registryErr, tc.settingsState, tc.registryIsReadOnly, false)
			c := config.New(ctx, dir, config.WithRegistry(r))

			err := c.FetchMicrosoftStoreSubscription(ctx)
			if tc.wantErr {
				require.Error(t, err, "FetchMicrosoftStoreSubscription should return an error")
			} else {
				require.NoError(t, err, "FetchMicrosoftStoreSubscription should return no errors")
			}

			// Disable errors so we can retrieve the token
			r.Errors = 0
			token, _, err := c.Subscription(ctx)
			require.NoError(t, err, "ProToken should return no error")
			require.Equal(t, tc.wantToken, token, "Unexpected value for ProToken")
		})
	}
}

func TestUpdateRegistrySettings(t *testing.T) {
	if wsl.MockAvailable() {
		t.Parallel()
	}

	testCases := map[string]struct {
		valueToChange string
		settingsState settingsState
		breakTaskfile bool

		wantTasks     []string
		unwantedTasks []string
		wantErr       bool
	}{
		"Success changing Pro token":               {valueToChange: "UbuntuProToken", settingsState: keyExists, wantTasks: []string{"tasks.ProAttachment"}, unwantedTasks: []string{"tasks.LandscapeConfigure"}},
		"Success changing Landscape without a UID": {valueToChange: "LandscapeConfig", settingsState: keyExists, unwantedTasks: []string{"tasks.ProAttachment", "tasks.LandscapeConfigure"}},
		"Success changing Landscape with a UID":    {valueToChange: "LandscapeConfig", settingsState: landscapeUIDHasValue, wantTasks: []string{"tasks.LandscapeConfigure"}, unwantedTasks: []string{"tasks.ProAttachment"}},
		"Success changing the Landscape UID":       {valueToChange: "LandscapeUID", settingsState: keyExists, wantTasks: []string{"tasks.LandscapeConfigure"}, unwantedTasks: []string{"tasks.ProAttachment"}},

		// Very implementation-detailed, but it's the only thing that actually triggers an error
		"Error when the tasks cannot be submitted": {valueToChange: "UbuntuProToken", settingsState: keyExists, breakTaskfile: true, wantErr: true},
	}

	for name, tc := range testCases {
		tc := tc
		t.Run(name, func(t *testing.T) {
			ctx := context.Background()
			if wsl.MockAvailable() {
				t.Parallel()
				ctx = wsl.WithMock(ctx, wslmock.New())
			}

			dir := t.TempDir()

			distroName, _ := wsltestutils.RegisterDistro(t, ctx, false)
			taskFilePath := filepath.Join(dir, distroName+".tasks")

			db, err := database.New(ctx, dir, nil)
			require.NoError(t, err, "Setup: could create empty database")

			_, err = db.GetDistroAndUpdateProperties(ctx, distroName, distro.Properties{})
			require.NoError(t, err, "Setup: could not add dummy distro to database")

			r, dir := setUpMockSettings(t, 0, tc.settingsState, false, false)
			require.NoError(t, err, "Setup: could not create empty database")

			c := config.New(ctx, dir, config.WithRegistry(r))

			// Update value in registry or in config
			if tc.valueToChange == "LandscapeUID" {
				err := c.SetLandscapeAgentUID(ctx, "NEW_UID!")
				require.NoError(t, err, "Setup: could not update Landscape UID")
			} else {
				r.UbuntuProData[tc.valueToChange] = "NEW_VALUE!"
			}

			if tc.breakTaskfile {
				err := os.MkdirAll(taskFilePath, 0600)
				require.NoError(t, err, "could not create directory to interfere with task file")
			}

			err = c.UpdateRegistrySettings(ctx, db)
			if tc.wantErr {
				require.Error(t, err, "UpdateRegistrySettings should return an error")
				return
			}
			require.NoError(t, err, "UpdateRegistrySettings should return no error")

			out, err := readFileOrEmpty(taskFilePath)
			require.NoError(t, err, "Could not read distro taskfile")
			for _, task := range tc.wantTasks {
				assert.Containsf(t, out, task, "Distro should have received a %s task", task)
			}
			for _, task := range tc.unwantedTasks {
				assert.NotContainsf(t, out, task, "Distro should have received a %s task", task)
			}
		})
	}
}

func readFileOrEmpty(path string) (string, error) {
	out, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return "", nil
	}
	return string(out), err
}

// is defines equality between flags. It is convenience function to check if a settingsState matches a certain state.
func (state settingsState) is(flag settingsState) bool {
	return state&flag == flag
}

func setUpMockSettings(t *testing.T, mockErrors uint32, state settingsState, readOnly bool, fileBroken bool) (*registry.Mock, string) {
	t.Helper()

	// Mock registry
	reg := registry.NewMock()
	reg.Errors = mockErrors
	reg.KeyIsReadOnly = readOnly

	if state.is(keyExists) {
		reg.KeyExists = true
	}

	if state.is(orgTokenExists) {
		reg.UbuntuProData["UbuntuProToken"] = ""
	}
	if state.is(orgTokenHasValue) {
		reg.UbuntuProData["UbuntuProToken"] = "org_token"
	}

	if state.is(orgLandscapeConfigExists) {
		reg.UbuntuProData["LandscapeConfig"] = ""
	}
	if state.is(orgLandscapeConfigHasValue) {
		reg.UbuntuProData["LandscapeConfig"] = "[client]\nuser=BigOrg"
	}

	// Mock file config
	cacheDir := t.TempDir()
	if fileBroken {
		err := os.MkdirAll(filepath.Join(cacheDir, "config"), 0600)
		require.NoError(t, err, "Setup: could not create directory to interfere with config")
		return reg, cacheDir
	}

	if !state.is(fileExists) {
		return reg, cacheDir
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

	return reg, cacheDir
}
