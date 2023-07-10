package ui_test

import (
	"context"
	"fmt"
	"reflect"
	"testing"

	agentapi "github.com/canonical/ubuntu-pro-for-windows/agentapi/go"
	"github.com/canonical/ubuntu-pro-for-windows/windows-agent/internal/config"
	"github.com/canonical/ubuntu-pro-for-windows/windows-agent/internal/config/registry"
	"github.com/canonical/ubuntu-pro-for-windows/windows-agent/internal/distros/database"
	"github.com/canonical/ubuntu-pro-for-windows/windows-agent/internal/distros/distro"
	"github.com/canonical/ubuntu-pro-for-windows/windows-agent/internal/proservices/ui"
	"github.com/canonical/ubuntu-pro-for-windows/windows-agent/internal/testutils"
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

	conf := config.New(ctx, config.WithRegistry(registry.NewMock()))

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

	distro1, _ := testutils.RegisterDistro(t, ctx, false)
	distro2, _ := testutils.RegisterDistro(t, ctx, false)

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
			m.UbuntuProData["ProTokenUser"] = originalToken

			conf := config.New(ctx, config.WithRegistry(m))
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

func TestGetSubscriptionInfo(t *testing.T) {
	t.Parallel()

	var (
		none         = reflect.TypeOf(&agentapi.SubscriptionInfo_None{}).String()
		user         = reflect.TypeOf(&agentapi.SubscriptionInfo_User{}).String()
		organization = reflect.TypeOf(&agentapi.SubscriptionInfo_Organization{}).String()
		store        = reflect.TypeOf(&agentapi.SubscriptionInfo_MicrosoftStore{}).String()
	)

	testCases := map[string]struct {
		registryReadOnly bool
		source           config.SubscriptionSource

		isReadOnlyErr   bool // Config errors out in IsReadOnly function
		subscriptionErr bool // Config errors out in Subscription function

		wantType      string
		wantImmutable bool
		wantErr       bool
	}{
		"Success with a non-subscription":           {source: config.SubscriptionNone, wantType: none},
		"Success with a read-only non-subscription": {source: config.SubscriptionNone, registryReadOnly: true, wantType: none, wantImmutable: true},

		"Success with an organization subscription":          {source: config.SubscriptionOrganization, wantType: organization},
		"Success with a read-only organization subscription": {source: config.SubscriptionOrganization, registryReadOnly: true, wantType: organization, wantImmutable: true},

		"Success with a user subscription":           {source: config.SubscriptionUser, wantType: user},
		"Success with a read-only user subscription": {source: config.SubscriptionUser, registryReadOnly: true, wantType: user, wantImmutable: true},

		"Success with a store subscription":           {source: config.SubscriptionMicrosoftStore, wantType: store},
		"Success with a read-only store subscription": {source: config.SubscriptionMicrosoftStore, registryReadOnly: true, wantType: store, wantImmutable: true},

		"Error when the the read-only check fails":        {isReadOnlyErr: true, wantErr: true},
		"Error when the subscription cannot be retreived": {subscriptionErr: true, wantErr: true},
	}

	for name, tc := range testCases {
		tc := tc
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			ctx := context.Background()

			dir := t.TempDir()
			db, err := database.New(ctx, dir, nil)
			require.NoError(t, err, "Setup: empty database New() should return no error")
			defer db.Close(ctx)

			m := registry.NewMock()
			m.KeyExists = true

			conf := config.New(ctx, config.WithRegistry(m))
			if tc.source != config.SubscriptionNone {
				err := conf.SetSubscription(ctx, "example_token", tc.source)
				require.NoError(t, err, "Setup: SetSubscription should return no error")
			}

			if tc.registryReadOnly {
				m.KeyIsReadOnly = true
			}

			if tc.isReadOnlyErr {
				m.Errors = registry.MockErrOnCreateKey
			}
			if tc.subscriptionErr {
				m.Errors |= registry.MockErrReadValue
			}

			service := ui.New(context.Background(), conf, db)

			info, err := service.GetSubscriptionInfo(ctx, &agentapi.Empty{})
			if tc.wantErr {
				require.Error(t, err, "GetSubscriptionInfo should return an error")
				return
			}
			require.NoError(t, err, "GetSubscriptionInfo should return no errors")

			require.Equal(t, tc.wantType, fmt.Sprintf("%T", info.SubscriptionType), "Mismatched subscription types")
			require.Equal(t, tc.wantImmutable, info.Immutable, "Mismatched value for ReadOnly")
		})
	}
}
