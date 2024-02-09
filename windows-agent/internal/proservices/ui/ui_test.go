package ui_test

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	agentapi "github.com/canonical/ubuntu-pro-for-wsl/agentapi/go"
	"github.com/canonical/ubuntu-pro-for-wsl/common/wsltestutils"
	"github.com/canonical/ubuntu-pro-for-wsl/windows-agent/internal/config"
	"github.com/canonical/ubuntu-pro-for-wsl/windows-agent/internal/contracts"
	"github.com/canonical/ubuntu-pro-for-wsl/windows-agent/internal/distros/database"
	"github.com/canonical/ubuntu-pro-for-wsl/windows-agent/internal/distros/distro"
	"github.com/canonical/ubuntu-pro-for-wsl/windows-agent/internal/proservices/ui"
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
	defer conf.Stop()

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
		tc := tc
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
			defer conf.Stop()

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
		tc := tc
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			ctx := context.Background()

			dir := t.TempDir()
			db, err := database.New(ctx, dir, nil)
			require.NoError(t, err, "Setup: empty database New() should return no error")
			service := ui.New(ctx, &tc.config, db)

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
		config mockConfig

		wantType      interface{}
		wantImmutable bool
		wantErr       bool
	}{
		"Success with a non-subscription":            {config: mockConfig{source: config.SourceNone}, wantType: store},
		"Success with an existing user subscription": {config: mockConfig{source: config.SourceUser}, wantType: store},

		"Error to fetch MS Store":                   {config: mockConfig{source: config.SourceNone, fetchErr: true}, wantType: none, wantErr: true},
		"Error to set the subscription":             {config: mockConfig{source: config.SourceNone, setSubscriptionErr: true}, wantType: none, wantErr: true},
		"Error to read the registry":                {config: mockConfig{source: config.SourceNone, subscriptionErr: true}, wantType: none, wantErr: true},
		"Error with an existing store subscription": {config: mockConfig{source: config.SourceMicrosoftStore}, wantType: store, wantErr: true},
	}

	for name, tc := range testCases {
		tc := tc
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			ctx := context.Background()

			dir := t.TempDir()
			db, err := database.New(ctx, dir, nil)
			require.NoError(t, err, "Setup: empty database New() should return no error")
			service := ui.New(ctx, &tc.config, db)

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

type mockConfig struct {
	setSubscriptionErr bool // Config errors out in SetSubscription function
	subscriptionErr    bool // Config errors out in Subscription function
	fetchErr           bool // Config errors out in FetchMicrosoftStoreSubscription function

	token  string        // stores the configured Pro token
	source config.Source // stores the configured subscription source.
}

func (m *mockConfig) SetUserSubscription(token string) error {
	if m.setSubscriptionErr {
		return errors.New("SetSubscription error")
	}
	m.token = token
	m.source = config.SourceUser
	return nil
}

func (m mockConfig) Subscription() (string, config.Source, error) {
	if m.subscriptionErr {
		return "", config.SourceNone, errors.New("Subscription error")
	}
	return m.token, m.source, nil
}

func (m *mockConfig) FetchMicrosoftStoreSubscription(ctx context.Context, args ...contracts.Option) error {
	if len(args) != 0 {
		panic("The variadic argument exists solely to match the interface. Do not use.")
	}

	if m.fetchErr {
		return errors.New("FetchMicrosoftStoreSubscription error")
	}
	if m.source == config.SourceMicrosoftStore {
		return errors.New("Already subscribed")
	}

	if m.setSubscriptionErr {
		return errors.New("SetSubscription error")
	}

	m.token = "MS"
	m.source = config.SourceMicrosoftStore

	return nil
}
