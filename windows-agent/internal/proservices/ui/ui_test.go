package ui_test

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	agentapi "github.com/canonical/ubuntu-pro-for-windows/agentapi/go"
	"github.com/canonical/ubuntu-pro-for-windows/common/wsltestutils"
	"github.com/canonical/ubuntu-pro-for-windows/windows-agent/internal/config"
	"github.com/canonical/ubuntu-pro-for-windows/windows-agent/internal/config/registry"
	"github.com/canonical/ubuntu-pro-for-windows/windows-agent/internal/distros/database"
	"github.com/canonical/ubuntu-pro-for-windows/windows-agent/internal/distros/distro"
	"github.com/canonical/ubuntu-pro-for-windows/windows-agent/internal/proservices/ui"
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

	conf := config.New(ctx, dir, config.WithRegistry(registry.NewMock()))

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
		distros          []string
		token            string
		registryReadOnly bool

		wantErr bool
	}{
		"No panic due empty token":          {token: ""},
		"Success with an empty database":    {token: "funny_token"},
		"Success with a non-empty database": {token: "whatever_token", distros: []string{distro1, distro2}},

		"Error due to no write permission on token": {registryReadOnly: true, wantErr: true},
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

			m := registry.NewMock()
			m.KeyIsReadOnly = tc.registryReadOnly
			m.KeyExists = true

			contents := fmt.Sprintf("subscription:\n  gui: %s", originalToken)
			err = os.WriteFile(filepath.Join(dir, "config"), []byte(contents), 0600)
			require.NoError(t, err, "Setup: could not write config file")

			conf := config.New(ctx, dir, config.WithRegistry(m))
			serv := ui.New(context.Background(), conf, db)

			info := agentapi.ProAttachInfo{Token: tc.token}
			_, err = serv.ApplyProToken(context.Background(), &info)

			var wantToken string
			if tc.wantErr {
				require.Error(t, err, "Unexpected success in ApplyProToken")
				wantToken = originalToken
			} else {
				require.NoError(t, err, "Adding the task to existing distros should succeed.")
				wantToken = tc.token
			}

			token, _, err := conf.Subscription(ctx)
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
		"Success with a non-subscription":           {config: mockConfig{source: config.SourceNone}, wantType: none},
		"Success with a read-only non-subscription": {config: mockConfig{source: config.SourceNone, registryReadOnly: true}, wantType: none, wantImmutable: true},

		"Success with an organization subscription":          {config: mockConfig{source: config.SourceRegistry}, wantType: organization},
		"Success with a read-only organization subscription": {config: mockConfig{source: config.SourceRegistry, registryReadOnly: true}, wantType: organization, wantImmutable: true},

		"Success with a user subscription":           {config: mockConfig{source: config.SourceGUI}, wantType: user},
		"Success with a read-only user subscription": {config: mockConfig{source: config.SourceGUI, registryReadOnly: true}, wantType: user, wantImmutable: true},

		"Success with a store subscription":           {config: mockConfig{source: config.SourceMicrosoftStore}, wantType: store},
		"Success with a read-only store subscription": {config: mockConfig{source: config.SourceMicrosoftStore, registryReadOnly: true}, wantType: store, wantImmutable: true},

		"Error when the read-only check fails":            {config: mockConfig{isReadOnlyErr: true}, wantErr: true},
		"Error when the subscription cannot be retreived": {config: mockConfig{subscriptionErr: true}, wantErr: true},
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
			require.Equal(t, tc.wantImmutable, info.GetImmutable(), "Mismatched value for ReadOnly")
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
		"Success with an existing user subscription": {config: mockConfig{source: config.SourceGUI}, wantType: store},

		"Error to fetch MS Store":                          {config: mockConfig{source: config.SourceNone, fetchErr: true}, wantType: none, wantErr: true},
		"Error to set the subscription":                    {config: mockConfig{source: config.SourceNone, setSubscriptionErr: true}, wantType: none, wantErr: true},
		"Error to read the registry":                       {config: mockConfig{source: config.SourceNone, isReadOnlyErr: true}, wantType: none, wantErr: true},
		"Error with an existing store subscription":        {config: mockConfig{source: config.SourceMicrosoftStore}, wantType: store, wantErr: true},
		"Error with a read-only organization subscription": {config: mockConfig{source: config.SourceRegistry, registryReadOnly: true}, wantType: organization, wantImmutable: true, wantErr: true},
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
			require.Equal(t, tc.wantImmutable, info.GetImmutable(), "Mismatched value for ReadOnly")
		})
	}
}

type mockConfig struct {
	registryReadOnly   bool // reports registry as read only
	setSubscriptionErr bool // Config errors out in SetSubscription function
	isReadOnlyErr      bool // Config errors out in IsReadOnly function
	subscriptionErr    bool // Config errors out in Subscription function
	fetchErr           bool // Config errors out in FetchMicrosoftStoreSubscription function

	token  string        // stores the configured Pro token
	source config.Source // stores the configured subscription source.
}

func (m *mockConfig) SetSubscription(ctx context.Context, token string, source config.Source) error {
	if m.setSubscriptionErr {
		return errors.New("SetSubscription error")
	}
	m.token = token
	m.source = source
	return nil
}
func (m mockConfig) IsReadOnly() (bool, error) {
	if m.isReadOnlyErr {
		return false, errors.New("IsReadOnly error")
	}
	return m.registryReadOnly, nil
}
func (m mockConfig) Subscription(context.Context) (string, config.Source, error) {
	if m.subscriptionErr {
		return "", config.SourceNone, errors.New("Subscription error")
	}
	return m.token, m.source, nil
}
func (m *mockConfig) FetchMicrosoftStoreSubscription(ctx context.Context) error {
	readOnly, err := m.IsReadOnly()
	if err != nil {
		return err
	}
	if readOnly {
		return errors.New("FetchMicrosoftStoreSubscription found read-only registry")
	}
	if m.fetchErr {
		return errors.New("FetchMicrosoftStoreSubscription error")
	}
	if m.source == config.SourceMicrosoftStore {
		return errors.New("Already subscribed")
	}

	return m.SetSubscription(ctx, "MS", config.SourceMicrosoftStore)
}
