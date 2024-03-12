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
	subsNone         = &agentapi.SubscriptionInfo_None{}
	subsOrganization = &agentapi.SubscriptionInfo_Organization{}
	subsUser         = &agentapi.SubscriptionInfo_User{}
	subsStore        = &agentapi.SubscriptionInfo_MicrosoftStore{}
)

var (
	lsNone         = &agentapi.LandscapeSource_None{}
	lsOrganization = &agentapi.LandscapeSource_Organization{}
	lsUser         = &agentapi.LandscapeSource_User{}
)

func TestGetConfigSources(t *testing.T) {
	t.Parallel()

	testCases := map[string]struct {
		config mockConfig

		wantSubscriptionType interface{}
		wantLandscapeType    interface{}
		wantErr              bool
	}{
		"Success with no config": {config: mockConfig{}, wantSubscriptionType: subsNone, wantLandscapeType: lsNone},

		"Success with an organization subscription": {config: mockConfig{proSource: config.SourceRegistry}, wantSubscriptionType: subsOrganization, wantLandscapeType: lsNone},
		"Success with a user subscription":          {config: mockConfig{proSource: config.SourceUser}, wantSubscriptionType: subsUser, wantLandscapeType: lsNone},
		"Success with a store subscription":         {config: mockConfig{proSource: config.SourceMicrosoftStore}, wantSubscriptionType: subsStore, wantLandscapeType: lsNone},

		"Success with a user Landscape source":          {config: mockConfig{landscapeSource: config.SourceUser}, wantSubscriptionType: subsNone, wantLandscapeType: lsUser},
		"Success with an organization Landscape source": {config: mockConfig{landscapeSource: config.SourceRegistry}, wantSubscriptionType: subsNone, wantLandscapeType: lsOrganization},

		"Error when the subscription cannot be retrieved":     {config: mockConfig{subscriptionErr: true}, wantErr: true},
		"Error when the Landscape source cannot be retrieved": {config: mockConfig{landscapeErr: true}, wantErr: true},
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

			src, err := service.GetConfigSources(ctx, &agentapi.Empty{})
			if tc.wantErr {
				require.Error(t, err, "GetConfigSources should return an error")
				return
			}
			require.NoError(t, err, "GetConfigSources should return no errors")

			info := src.GetProSubscription()
			require.IsType(t, tc.wantSubscriptionType, info.GetSubscriptionType(), "Mismatched subscription types")

			l := src.GetLandscapeSource()
			require.IsType(t, tc.wantLandscapeType, l.GetLandscapeSourceType(), "Mismatched Landscape source types")
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
		"Success with a non-subscription":            {wantType: subsStore},
		"Success with an existing user subscription": {haveUserToken: true, wantType: subsStore},

		"Error when FetchMicrosoftStoreSubscription returns an error": {breakConfig: true, wantType: subsNone, wantErr: true},
		"Error when the subscription source is unknown":               {breakConfigSource: true, wantType: subsNone, wantErr: true},
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
				conf.proSource = config.SourceUser
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
		landscapeSource           config.Source

		wantErr bool
		want    interface{}
	}{
		"Success": {want: lsUser},

		"Error when setting the config returns error":  {setUserLandscapeConfigErr: true, wantErr: true},
		"Error when attempting to override org config": {landscapeSource: config.SourceRegistry, wantErr: true},
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
				landscapeSource:           tc.landscapeSource,
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

			require.IsType(t, tc.want, got.GetLandscapeSourceType(), "Mismatched Landscape source types")
			require.Equal(t, landscapeConfig, conf.gotLandscapeConfig, "Config received unexpected Landscape config")
		})
	}
}

type mockConfig struct {
	setUserSubscriptionErr    bool // Config errors out in SetUserSubscription function
	subscriptionErr           bool // Config errors out in Subscription function
	setUserLandscapeConfigErr bool // Config errors out in SetUserLandscapeConfig function
	landscapeErr              bool // Config errors out in LandscapeClientConfig function

	token           string        // stores the configured Pro token
	proSource       config.Source // stores the configured subscription source.
	landscapeSource config.Source // stores the configured landscape source.

	returnBadSource    bool
	gotLandscapeConfig string
}

func (m *mockConfig) SetUserSubscription(ctx context.Context, token string) error {
	if m.setUserSubscriptionErr {
		return errors.New("SetUserSubscription: mock error")
	}
	m.token = token
	m.proSource = config.SourceUser
	return nil
}

func (m *mockConfig) SetStoreSubscription(ctx context.Context, token string) error {
	m.token = token
	m.proSource = config.SourceMicrosoftStore
	return nil
}

func (m *mockConfig) SetUserLandscapeConfig(ctx context.Context, landscapeConfig string) error {
	if m.setUserLandscapeConfigErr {
		return errors.New("mock error")
	}

	if m.landscapeSource == config.SourceRegistry {
		return errors.New("mock error cannot overwrite organization's configuration data")
	}

	m.gotLandscapeConfig = landscapeConfig
	m.landscapeSource = config.SourceUser

	return nil
}

func (m mockConfig) Subscription() (string, config.Source, error) {
	if m.subscriptionErr {
		return "", config.SourceNone, errors.New("Subscription error")
	}
	if m.returnBadSource {
		return "", config.Source(100000), nil
	}
	return m.token, m.proSource, nil
}

func (m mockConfig) LandscapeClientConfig() (string, config.Source, error) {
	if m.landscapeErr {
		return "", config.SourceNone, errors.New("LandscapeClientConfig error")
	}
	if m.returnBadSource {
		return "", config.Source(100000), nil
	}
	return "[host]", m.landscapeSource, nil
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
