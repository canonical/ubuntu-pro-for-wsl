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
type registryState uint64

const (
	untouched registryState = 0x00 // Nothing UbuntuPro-related exists, as though the program had never ran before
	keyExists registryState = 0x01 // Key exists but is empty

	orgTokenExists              = keyExists | 1<<(iota+2) // Key exists, organization token field exists
	userTokenExists                                       // Key exists, user token field exists
	storeTokenExists                                      // Key exists, microsoft store token field exists
	landscapeAgentURLExists                               // Key exists, landscape agent URL field exists
	landscapeClientConfigExists                           // Key exists, landscape client config field exists
	landscapeAgentUIDExists                               // Key exists, landscape agent UID field exists

	orgTokenHasValue              = orgTokenExists | 1<<16              // Key exists, organization token field exists and is not empty
	userTokenHasValue             = userTokenExists | 1<<17             // Key exists, user token field exists and is not empty
	storeTokenHasValue            = storeTokenExists | 1<<18            // Key exists, microsoft store token field exists and is not empty
	landscapeAgentURLHasValue     = landscapeAgentURLExists | 1<<19     // Key exists,  landscape agent URL field exists and is not empty
	landscapeClientConfigHasValue = landscapeClientConfigExists | 1<<20 // Key exists, landscape client config field exists and is not empty
	landscapeAgentUIDHasValue     = landscapeAgentUIDExists | 1<<21     // Key exists, landscape agent UID field exists and is not empty
)

func TestSubscription(t *testing.T) {
	t.Parallel()

	testCases := map[string]struct {
		mockErrors    uint32
		registryState registryState

		wantToken  string
		wantSource config.SubscriptionSource
		wantError  bool
	}{
		"Success":                                               {registryState: userTokenHasValue, wantToken: "user_token", wantSource: config.SubscriptionUser},
		"Success when the key does not exist":                   {registryState: untouched},
		"Success when the key exists but is empty":              {registryState: keyExists},
		"Success when the key exists but contains empty fields": {registryState: orgTokenExists | userTokenExists | storeTokenExists},

		"Success when there is an organization token": {registryState: orgTokenHasValue, wantToken: "org_token", wantSource: config.SubscriptionOrganization},
		"Success when there is a user token":          {registryState: userTokenHasValue, wantToken: "user_token", wantSource: config.SubscriptionUser},
		"Success when there is a store token":         {registryState: storeTokenHasValue, wantToken: "store_token", wantSource: config.SubscriptionMicrosoftStore},

		"Success when there are organization and user tokens":                           {registryState: orgTokenHasValue | userTokenHasValue, wantToken: "user_token", wantSource: config.SubscriptionUser},
		"Success when there are organization and store tokens":                          {registryState: orgTokenHasValue | storeTokenHasValue, wantToken: "store_token", wantSource: config.SubscriptionMicrosoftStore},
		"Success when there are organization and user tokens, and an empty store token": {registryState: orgTokenHasValue | userTokenHasValue | storeTokenExists, wantToken: "user_token", wantSource: config.SubscriptionUser},

		"Error when the registry key cannot be opened":    {registryState: userTokenHasValue, mockErrors: registry.MockErrOnOpenKey, wantError: true},
		"Error when the registry key cannot be read from": {registryState: userTokenHasValue, mockErrors: registry.MockErrReadValue, wantError: true},
	}

	for name, tc := range testCases {
		tc := tc
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			ctx := context.Background()

			r := setUpMockRegistry(tc.mockErrors, tc.registryState, false)
			conf := config.New(ctx, config.WithRegistry(r))

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

func TestLandscapeAgentURL(t *testing.T) {
	t.Parallel()
	testConfigGetter(t, testConfigGetterSettings{
		getter:           (*config.Config).LandscapeAgentURL,
		getterName:       "LandscapeAgentURL",
		registryHasValue: landscapeAgentURLHasValue,
		want:             "www.example.com/registry-example",
	})
}

func TestLandscapeClientConfig(t *testing.T) {
	t.Parallel()

	testConfigGetter(t, testConfigGetterSettings{
		getter:           (*config.Config).LandscapeClientConfig,
		getterName:       "LandscapeClientConfig",
		registryHasValue: landscapeClientConfigHasValue,
		want:             "[client]\nuser=JohnDoe",
	})
}

func TestLandscapeAgentUID(t *testing.T) {
	t.Parallel()

	testConfigGetter(t, testConfigGetterSettings{
		getter:           (*config.Config).LandscapeAgentUID,
		getterName:       "LandscapeAgentUID",
		registryHasValue: landscapeAgentUIDHasValue,
		want:             "landscapeUID1234",
	})
}

type testConfigGetterSettings struct {
	getter           func(*config.Config, context.Context) (string, error)
	getterName       string
	registryHasValue registryState
	want             string
}

//nolint:thelper // This is the test itself, not a helper. Besides, a t.Helper() here would not affect the subtests.
func testConfigGetter(t *testing.T, s testConfigGetterSettings) {
	testCases := map[string]struct {
		mockErrors    uint32
		registryState registryState

		wantError bool
	}{
		"Success":                               {registryState: s.registryHasValue},
		"Success when the key does not exist":   {registryState: untouched},
		"Success when the value does not exist": {registryState: keyExists},

		"Error when the registry key cannot be opened":    {registryState: s.registryHasValue, mockErrors: registry.MockErrOnOpenKey, wantError: true},
		"Error when the registry key cannot be read from": {registryState: s.registryHasValue, mockErrors: registry.MockErrReadValue, wantError: true},
	}

	for name, tc := range testCases {
		tc := tc
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			ctx := context.Background()

			r := setUpMockRegistry(tc.mockErrors, tc.registryState, false)
			conf := config.New(ctx, config.WithRegistry(r))

			v, err := s.getter(conf, ctx)
			if tc.wantError {
				require.Error(t, err, "%s should return an error", s.getterName)
				return
			}
			require.NoError(t, err, "%s should return no error", s.getterName)

			// Test default values
			if !tc.registryState.is(s.registryHasValue) {
				require.Emptyf(t, v, "Unexpected value when %s is not set in registry", s.getterName)
				return
			}

			// Test non-default values
			assert.Equalf(t, s.want, v, "%s returned an unexpected value", s.getterName)
			assert.Zerof(t, r.OpenKeyCount.Load(), "Call to %s leaks registry keys", s.getterName)
		})
	}
}

func TestProvisioningTasks(t *testing.T) {
	t.Parallel()

	testCases := map[string]struct {
		mockErrors    uint32
		registryState registryState

		want            string
		wantNoLandscape bool
		wantError       bool
	}{
		"Success when the key does not exist":                {registryState: untouched},
		"Success when the pro token field does not exist":    {registryState: keyExists},
		"Success when the pro token exists but is empty":     {registryState: userTokenExists},
		"Success with a user token":                          {registryState: userTokenHasValue, want: "user_token"},
		"Success when there is Landscape config, but no UID": {registryState: landscapeClientConfigHasValue, wantNoLandscape: true},
		"Success when there is Landscape config and UID":     {registryState: landscapeClientConfigHasValue | landscapeAgentUIDHasValue},

		"Error when the registry key cannot be opened":    {registryState: userTokenExists, mockErrors: registry.MockErrOnOpenKey, wantError: true},
		"Error when the registry key cannot be read from": {registryState: userTokenExists, mockErrors: registry.MockErrReadValue, wantError: true},
	}

	for name, tc := range testCases {
		tc := tc
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			ctx := context.Background()

			r := setUpMockRegistry(tc.mockErrors, tc.registryState, false)
			conf := config.New(ctx, config.WithRegistry(r))

			gotTasks, err := conf.ProvisioningTasks(ctx, "UBUNTU")
			if tc.wantError {
				require.Error(t, err, "ProvisioningTasks should return an error")
				return
			}
			require.NoError(t, err, "ProvisioningTasks should return no error")

			wantTasks := []task.Task{
				tasks.ProAttachment{Token: tc.want},
			}

			if !tc.wantNoLandscape {
				wantTasks = append(wantTasks, tasks.LandscapeConfigure{
					Config:       r.UbuntuProData["LandscapeClientConfig"],
					HostagentUID: r.UbuntuProData["LandscapeAgentUID"],
				})
			}

			require.ElementsMatch(t, wantTasks, gotTasks, "Unexpected contents returned by ProvisioningTasks")
		})
	}
}

func TestSetSubscription(t *testing.T) {
	t.Parallel()

	testCases := map[string]struct {
		mockErrors       uint32
		registryState    registryState
		accessIsReadOnly bool
		emptyToken       bool

		want          string
		wantError     bool
		wantErrorType error
	}{
		"Success":                                         {registryState: userTokenHasValue, want: "new_token"},
		"Success disabling a subscription":                {registryState: userTokenHasValue, emptyToken: true, want: ""},
		"Success when the key does not exist":             {registryState: untouched, want: "new_token"},
		"Success when the pro token field does not exist": {registryState: keyExists, want: "new_token"},
		"Success when there is a store token active":      {registryState: storeTokenHasValue, want: "store_token"},

		"Error when the registry key cannot be written on due to lack of permission": {registryState: userTokenHasValue, accessIsReadOnly: true, want: "user_token", wantError: true, wantErrorType: registry.ErrAccessDenied},
		"Error when the registry key cannot be opened":                               {registryState: userTokenHasValue, mockErrors: registry.MockErrOnCreateKey, want: "user_token", wantError: true, wantErrorType: registry.ErrMock},
		"Error when the registry key cannot be written on":                           {registryState: userTokenHasValue, mockErrors: registry.MockErrOnWriteValue, want: "user_token", wantError: true, wantErrorType: registry.ErrMock},
		"Error when the registry key cannot be read":                                 {registryState: userTokenHasValue, mockErrors: registry.MockErrOnOpenKey, want: "user_token", wantError: true, wantErrorType: registry.ErrMock},
	}

	for name, tc := range testCases {
		tc := tc
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			ctx := context.Background()

			r := setUpMockRegistry(tc.mockErrors, tc.registryState, tc.accessIsReadOnly)
			conf := config.New(ctx, config.WithRegistry(r))

			token := "new_token"
			if tc.emptyToken {
				token = ""
			}

			err := conf.SetSubscription(ctx, token, config.SubscriptionUser)
			if tc.wantError {
				require.Error(t, err, "SetSubscription should return an error")
				if tc.wantErrorType != nil {
					require.ErrorIs(t, err, tc.wantErrorType, "SetSubscription returned an error of unexpected type")
				}
			} else {
				require.NoError(t, err, "SetSubscription should return no error")
			}

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
		mockErrors       uint32
		registryState    registryState
		accessIsReadOnly bool
		emptyUID         bool

		want          string
		wantError     bool
		wantErrorType error
	}{
		"Success":                                         {registryState: landscapeAgentUIDHasValue, want: "new_uid"},
		"Success unsetting the UID":                       {registryState: landscapeAgentUIDHasValue, emptyUID: true, want: ""},
		"Success when the key does not exist":             {registryState: untouched, want: "new_uid"},
		"Success when the pro token field does not exist": {registryState: keyExists, want: "new_uid"},

		"Error when the registry key cannot be written on due to lack of permission": {registryState: landscapeAgentUIDHasValue, accessIsReadOnly: true, want: "landscapeUID1234", wantError: true, wantErrorType: registry.ErrAccessDenied},
		"Error when the registry key cannot be opened":                               {registryState: landscapeAgentUIDHasValue, mockErrors: registry.MockErrOnCreateKey, want: "landscapeUID1234", wantError: true, wantErrorType: registry.ErrMock},
		"Error when the registry key cannot be written on":                           {registryState: landscapeAgentUIDHasValue, mockErrors: registry.MockErrOnWriteValue, want: "landscapeUID1234", wantError: true, wantErrorType: registry.ErrMock},
		"Error when the registry key cannot be read":                                 {registryState: landscapeAgentUIDHasValue, mockErrors: registry.MockErrOnOpenKey, want: "landscapeUID1234", wantError: true, wantErrorType: registry.ErrMock},
	}

	for name, tc := range testCases {
		tc := tc
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			ctx := context.Background()

			r := setUpMockRegistry(tc.mockErrors, tc.registryState, tc.accessIsReadOnly)
			conf := config.New(ctx, config.WithRegistry(r))

			uid := "new_uid"
			if tc.emptyUID {
				uid = ""
			}

			err := conf.SetLandscapeAgentUID(ctx, uid)
			if tc.wantError {
				require.Error(t, err, "SetLandscapeAgentUID should return an error")
				if tc.wantErrorType != nil {
					require.ErrorIs(t, err, tc.wantErrorType, "SetLandscapeAgentUID returned an error of unexpected type")
				}
			} else {
				require.NoError(t, err, "SetLandscapeAgentUID should return no error")
			}

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
		registryState registryState
		readOnly      bool
		registryErr   bool

		want    bool
		wantErr bool
	}{
		"Success when the registry can be written on":    {registryState: keyExists, want: false},
		"Success when the registry cannot be written on": {registryState: keyExists, readOnly: true, want: true},

		"Success when the non-existent registry can be written on":    {want: false},
		"Success when the non-existent registry cannot be written on": {readOnly: true, want: true},

		"Error when the registry cannot be queried": {registryErr: true, wantErr: true},
	}

	for name, tc := range testCases {
		tc := tc
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			ctx := context.Background()

			r := setUpMockRegistry(0, tc.registryState, tc.readOnly)
			if tc.registryErr {
				r.Errors = registry.MockErrOnCreateKey
			}

			conf := config.New(ctx, config.WithRegistry(r))

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
		registryState      registryState
		registryErr        uint32
		registryIsReadOnly bool

		wantToken string
		wantErr   bool
	}{
		// TODO: Implement more test cases when the MS Store mock is available. There is no single successful test in here so far.
		"Error when registry is read only":          {registryState: userTokenHasValue, registryIsReadOnly: true, wantToken: "user_token", wantErr: true},
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

			r := setUpMockRegistry(tc.registryErr, tc.registryState, tc.registryIsReadOnly)
			c := config.New(ctx, config.WithRegistry(r))

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

// is defines equality between flags. It is convenience function to check if a registryState matches a certain state.
func (state registryState) is(flag registryState) bool {
	return state&flag == flag
}

func setUpMockRegistry(mockErrors uint32, state registryState, readOnly bool) *registry.Mock {
	r := registry.NewMock()

	r.Errors = mockErrors
	r.KeyIsReadOnly = readOnly

	if state.is(keyExists) {
		r.KeyExists = true
	}

	if state.is(orgTokenExists) {
		r.UbuntuProData["ProTokenOrg"] = ""
	}
	if state.is(orgTokenHasValue) {
		r.UbuntuProData["ProTokenOrg"] = "org_token"
	}

	if state.is(userTokenExists) {
		r.UbuntuProData["ProTokenUser"] = ""
	}
	if state.is(userTokenHasValue) {
		r.UbuntuProData["ProTokenUser"] = "user_token"
	}

	if state.is(storeTokenExists) {
		r.UbuntuProData["ProTokenStore"] = ""
	}
	if state.is(storeTokenHasValue) {
		r.UbuntuProData["ProTokenStore"] = "store_token"
	}

	if state.is(landscapeAgentURLExists) {
		r.UbuntuProData["LandscapeAgentURL"] = ""
	}
	if state.is(landscapeAgentURLHasValue) {
		r.UbuntuProData["LandscapeAgentURL"] = "www.example.com/registry-example"
	}

	if state.is(landscapeClientConfigExists) {
		r.UbuntuProData["LandscapeClientConfig"] = ""
	}
	if state.is(landscapeClientConfigHasValue) {
		r.UbuntuProData["LandscapeClientConfig"] = "[client]\nuser=JohnDoe"
	}

	if state.is(landscapeAgentUIDExists) {
		r.UbuntuProData["LandscapeAgentUID"] = ""
	}
	if state.is(landscapeAgentUIDHasValue) {
		r.UbuntuProData["LandscapeAgentUID"] = "landscapeUID1234"
	}

	return r
}
