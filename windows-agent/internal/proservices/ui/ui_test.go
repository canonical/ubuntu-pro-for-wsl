package ui_test

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"testing"
	"time"

	agentapi "github.com/canonical/ubuntu-pro-for-wsl/agentapi/go"
	"github.com/canonical/ubuntu-pro-for-wsl/common/wsltestutils"
	"github.com/canonical/ubuntu-pro-for-wsl/mocks/contractserver/contractsmockserver"
	"github.com/canonical/ubuntu-pro-for-wsl/windows-agent/internal/config"
	"github.com/canonical/ubuntu-pro-for-wsl/windows-agent/internal/distros/database"
	"github.com/canonical/ubuntu-pro-for-wsl/windows-agent/internal/distros/distro"
	"github.com/canonical/ubuntu-pro-for-wsl/windows-agent/internal/proservices/ui"
	"github.com/canonical/ubuntu-pro-for-wsl/windows-agent/internal/ubuntupro/contracts"
	"github.com/stretchr/testify/require"
	wsl "github.com/ubuntu/gowsl"
	wslmock "github.com/ubuntu/gowsl/mock"
)

func TestNew(t *testing.T) {
	ctx := context.Background()
	t.Parallel()

	dir := t.TempDir()
	db, err := database.New(ctx, dir, nil)
	require.NoError(t, err, "Setup: empty database New() should return no error")
	defer db.Close(ctx)

	conf := config.New(ctx, dir)

	_ = ui.New(context.Background(), conf, db)
}

// Subtests are parallel but the test itself is not due to the calls to RegisterDistro.
//
//nolint:tparallel
func TestAttachPro(t *testing.T) {
	ctx := context.Background()
	if wsl.MockAvailable() {
		t.Parallel()
		ctx = wsl.WithMock(ctx, wslmock.New())
	}

	distro1, _ := wsltestutils.RegisterDistro(t, ctx, false)
	distro2, _ := wsltestutils.RegisterDistro(t, ctx, false)

	testCases := map[string]struct {
		distros             []string
		token               string
		breakConfig         bool
		higherPriorityToken bool

		wantErr bool
	}{
		"No panic due empty token":          {token: ""},
		"Success with an empty database":    {token: "funny_token"},
		"Success with a non-empty database": {token: "whatever_token", distros: []string{distro1, distro2}},

		"Error when the config cannot write":                  {breakConfig: true, wantErr: true},
		"Error when there already is a higher priority token": {higherPriorityToken: true, wantErr: true},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			dir := t.TempDir()
			db, err := database.New(ctx, dir, nil)
			require.NoError(t, err, "Setup: empty database New() should return no error")
			defer db.Close(ctx)

			// Populate the database
			for i := range tc.distros {
				d, err := db.GetDistroAndUpdateProperties(context.Background(), tc.distros[i], distro.Properties{})
				require.NoError(t, err, "Setup: could not add %q to database", tc.distros[i])
				defer d.Cleanup(ctx)
			}

			const originalToken = "old_token"

			if tc.breakConfig {
				err := os.MkdirAll(filepath.Join(dir, "config"), 0600)
				require.NoError(t, err, "Setup: could not create directory to interfere with config")
			} else {
				contents := fmt.Sprintf("subscription:\n  gui: %s", originalToken)
				err = os.WriteFile(filepath.Join(dir, "config"), []byte(contents), 0600)
				require.NoError(t, err, "Setup: could not write config file")
			}

			conf := config.New(ctx, dir)

			if tc.higherPriorityToken {
				err = conf.UpdateRegistryData(ctx, config.RegistryData{
					UbuntuProToken: "organization_token",
				}, db)
				require.NoError(t, err, "Setup: could not make registry read registry settings")
			}

			serv := ui.New(context.Background(), conf, db)

			info := agentapi.ProAttachInfo{Token: tc.token}
			_, err = serv.ApplyProToken(context.Background(), &info)

			var wantToken string
			if tc.wantErr {
				require.Error(t, err, "Unexpected success in ApplyProToken")
				return
			}
			require.NoError(t, err, "Adding the task to existing distros should succeed.")
			wantToken = tc.token

			token, _, err := conf.Subscription()
			require.NoError(t, err, "conf.ProToken should return no error")
			require.Equal(t, wantToken, token, "unexpected active token")
		})
	}
}

var (
	none         = &agentapi.SubscriptionInfo_None{}
	user         = &agentapi.SubscriptionInfo_User{}
	organization = &agentapi.SubscriptionInfo_Organization{}
	store        = &agentapi.SubscriptionInfo_MicrosoftStore{}
)

func TestGetSubscriptionInfo(t *testing.T) {
	t.Parallel()

	testCases := map[string]struct {
		config mockConfig

		wantType      interface{}
		wantImmutable bool
		wantErr       bool
	}{
		"Success with a non-subscription":                 {config: mockConfig{source: config.SourceNone}, wantType: none},
		"Success with an organization subscription":       {config: mockConfig{source: config.SourceRegistry}, wantType: organization},
		"Success with a user subscription":                {config: mockConfig{source: config.SourceUser}, wantType: user},
		"Success with a store subscription":               {config: mockConfig{source: config.SourceMicrosoftStore}, wantType: store},
		"Error when the subscription cannot be retrieved": {config: mockConfig{subscriptionErr: true}, wantErr: true},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			ctx := context.Background()

			dir := t.TempDir()
			db, err := database.New(ctx, dir, nil)
			require.NoError(t, err, "Setup: empty database New() should return no error")
			config := tc.config
			service := ui.New(ctx, &config, db)

			info, err := service.GetSubscriptionInfo(ctx, &agentapi.Empty{})
			if tc.wantErr {
				require.Error(t, err, "GetSubscriptionInfo should return an error")
				return
			}
			require.NoError(t, err, "GetSubscriptionInfo should return no errors")

			require.IsType(t, tc.wantType, info.GetSubscriptionType(), "Mismatched subscription types")
		})
	}
}

func TestNotifyPurchase(t *testing.T) {
	t.Parallel()

	testCases := map[string]struct {
		haveUserToken     bool
		breakConfig       bool
		breakConfigSource bool

		wantType      interface{}
		wantImmutable bool
		wantErr       bool
	}{
		"Success with a non-subscription":            {wantType: store},
		"Success with an existing user subscription": {haveUserToken: true, wantType: store},

		"Error when FetchMicrosoftStoreSubscription returns an error": {breakConfig: true, wantType: none, wantErr: true},
		"Error when the subscription source is unknown":               {breakConfigSource: true, wantType: none, wantErr: true},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			ctx := context.Background()

			dir := t.TempDir()
			db, err := database.New(ctx, dir, nil)
			require.NoError(t, err, "Setup: empty database New() should return no error")

			opts, stop := setupMockContracts(t, ctx)
			defer stop()

			conf := &mockConfig{
				subscriptionErr: tc.breakConfig,
				returnBadSource: tc.breakConfigSource,
			}
			if tc.haveUserToken {
				conf.token = "USER_TOKEN"
				conf.source = config.SourceUser
			}

			service := ui.New(ctx, conf, db, opts...)
			info, err := service.NotifyPurchase(ctx, &agentapi.Empty{})
			if tc.wantErr {
				require.Error(t, err, "NotifyPurchase should return an error")
				return
			}
			require.NoError(t, err, "NotifyPurchase should return no errors")

			require.IsType(t, tc.wantType, info.GetSubscriptionType(), "Mismatched subscription types")
		})
	}
}

func TestApplyLandscapeConfig(t *testing.T) {
	t.Parallel()

	testCases := map[string]struct {
		setUserLandscapeConfigErr bool

		wantErr bool
	}{
		"Success": {},

		"Error when setting the config returns error": {setUserLandscapeConfigErr: true, wantErr: true},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			ctx := context.Background()

			landscapeConfig := "look at me! I am a Landscape config"

			dir := t.TempDir()
			db, err := database.New(ctx, dir, nil)
			require.NoError(t, err, "Setup: empty database New() should return no error")
			defer db.Close(ctx)

			conf := &mockConfig{
				setUserLandscapeConfigErr: tc.setUserLandscapeConfigErr,
			}

			uiService := ui.New(context.Background(), conf, db)

			msg := &agentapi.LandscapeConfig{
				Config: landscapeConfig,
			}

			got, err := uiService.ApplyLandscapeConfig(ctx, msg)
			if tc.wantErr {
				require.Error(t, err, "ApplyLandscapeConfig should return an error")
				return
			}
			require.NoError(t, err, "ApplyLandscapeConfig should return no errors")

			require.NotNil(t, got, "ApplyLandscapeConfig should not return a nil ageEmpty")
			require.Equal(t, landscapeConfig, conf.gotLandscapeConfig, "Config received unexpected Landscape config")
		})
	}
}

type mockConfig struct {
	setUserSubscriptionErr    bool // Config errors out in SetUserSubscription function
	subscriptionErr           bool // Config errors out in Subscription function
	setUserLandscapeConfigErr bool // Config errors out in SetUserLandscapeConfig function

	token  string        // stores the configured Pro token
	source config.Source // stores the configured subscription source.

	returnBadSource    bool
	gotLandscapeConfig string
}

func (m *mockConfig) SetUserSubscription(ctx context.Context, token string) error {
	if m.setUserSubscriptionErr {
		return errors.New("SetUserSubscription: mock error")
	}
	m.token = token
	m.source = config.SourceUser
	return nil
}

func (m *mockConfig) SetStoreSubscription(ctx context.Context, token string) error {
	m.token = token
	m.source = config.SourceMicrosoftStore
	return nil
}

func (m *mockConfig) SetUserLandscapeConfig(ctx context.Context, landscapeConfig string) error {
	if m.setUserLandscapeConfigErr {
		return errors.New("mock error")
	}

	m.gotLandscapeConfig = landscapeConfig

	return nil
}

func (m mockConfig) Subscription() (string, config.Source, error) {
	if m.subscriptionErr {
		return "", config.SourceNone, errors.New("Subscription error")
	}
	if m.returnBadSource {
		return "", config.Source(100000), nil
	}
	return m.token, m.source, nil
}

//nolint:revive // Testing t comes before the context.
func setupMockContracts(t *testing.T, ctx context.Context) (opts []contracts.Option, stop func()) {
	t.Helper()

	csSettings := contractsmockserver.DefaultSettings()
	server := contractsmockserver.NewServer(csSettings)

	err := server.Serve(ctx, "localhost:0")
	require.NoError(t, err, "Setup: Server should return no error")

	csAddr, err := url.Parse(fmt.Sprintf("http://%s", server.Address()))
	require.NoError(t, err, "Setup: Server URL should have been parsed with no issues")

	opts = []contracts.Option{
		contracts.WithProURL(csAddr),
		contracts.WithMockMicrosoftStore(mockMSStore{}),
	}

	return opts, func() { _ = server.Stop() }
}

type mockMSStore struct{}

func (s mockMSStore) GenerateUserJWT(azureADToken string) (jwt string, err error) {
	return "JWT", nil
}

func (s mockMSStore) GetSubscriptionExpirationDate() (tm time.Time, err error) {
	return time.Now().Add(time.Hour), nil
}
