package ubuntupro_test

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"testing"
	"time"

	"github.com/canonical/ubuntu-pro-for-wsl/common/wsltestutils"
	"github.com/canonical/ubuntu-pro-for-wsl/mocks/contractserver/contractsmockserver"
	"github.com/canonical/ubuntu-pro-for-wsl/windows-agent/internal/config"
	"github.com/canonical/ubuntu-pro-for-wsl/windows-agent/internal/distros/database"
	"github.com/canonical/ubuntu-pro-for-wsl/windows-agent/internal/distros/distro"
	"github.com/canonical/ubuntu-pro-for-wsl/windows-agent/internal/ubuntupro"
	"github.com/canonical/ubuntu-pro-for-wsl/windows-agent/internal/ubuntupro/contracts"
	"github.com/stretchr/testify/require"
	wsl "github.com/ubuntu/gowsl"
	wslmock "github.com/ubuntu/gowsl/mock"
)

func TestDistribute(t *testing.T) {
	if wsl.MockAvailable() {
		t.Parallel()
	}

	testCases := map[string]struct {
		distroIsDead bool
		breakConfig  bool

		wantErr bool
	}{
		"Success": {},
		"Success when a task cannot be submitted": {distroIsDead: true},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			ctx := context.Background()
			if wsl.MockAvailable() {
				t.Parallel()
				ctx = wsl.WithMock(ctx, wslmock.New())
			}

			db, err := database.New(ctx, t.TempDir())
			require.NoError(t, err, "Setup: Database creation should return no error")

			distroName, _ := wsltestutils.RegisterDistro(t, ctx, false)

			dist, err := db.GetDistroAndUpdateProperties(ctx, distroName, distro.Properties{})
			require.NoError(t, err, "Setup: GetDistroAndUpdateProperties should return no error")
			defer dist.Cleanup(ctx)

			if tc.distroIsDead {
				dist.Invalidate(ctx)
			}

			ubuntupro.Distribute(ctx, db, "super_token")
		})
	}
}

func TestFetchFromMicrosoftStore(t *testing.T) {
	if wsl.MockAvailable() {
		t.Parallel()
	}

	//nolint:gosec // These are not real credentials
	const (
		oldProToken  = "OLD_UBUNTU_PRO_TOKEN"
		proToken     = "UBUNTU_PRO_TOKEN_456"
		azureADToken = "AZURE_AD_TOKEN_789"
	)

	testCases := map[string]struct {
		breakSubscription     bool
		breakSetStoreProToken bool

		alreadyHaveToken    bool
		hasOrgToken         bool
		subscriptionExpired bool

		msStoreJWTErr        bool
		msStoreExpirationErr bool

		wantToken string
		wantErr   bool
	}{
		"Success": {wantToken: proToken},
		"Success when there is a store token already":  {alreadyHaveToken: true, wantToken: oldProToken},
		"Success when there is an organization token":  {hasOrgToken: true, wantToken: oldProToken},
		"Success when there is an expired store token": {alreadyHaveToken: true, subscriptionExpired: true, wantToken: proToken},

		// Config errors
		"Error when the current subscription cannot be obtained": {breakSubscription: true, wantErr: true},
		"Error when the new subscription cannot be set":          {breakSetStoreProToken: true, wantErr: true},

		// Contract server errors
		"Error when the Microsoft Store cannot provide the JWT":             {msStoreJWTErr: true, wantErr: true},
		"Error when the Microsoft Store cannot provide the expiration date": {alreadyHaveToken: true, msStoreExpirationErr: true, wantErr: true},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			ctx := context.Background()
			if wsl.MockAvailable() {
				t.Parallel()
				ctx = wsl.WithMock(ctx, wslmock.New())
			}

			conf := &mockConfig{
				subscriptionErr:     tc.breakSubscription,
				setStoreProTokenErr: tc.breakSetStoreProToken,
			}

			if tc.hasOrgToken {
				conf.registryProToken = oldProToken
			}

			if tc.alreadyHaveToken {
				conf.storeProToken = oldProToken
			}

			// Set up the mock Microsoft store
			store := mockMSStore{
				expirationDate:    time.Now().Add(24 * 365 * time.Hour), // Next year
				expirationDateErr: tc.msStoreExpirationErr,

				jwt:    "JWT_123",
				jwtErr: tc.msStoreJWTErr,
			}

			if tc.subscriptionExpired {
				store.expirationDate = time.Now().Add(-24 * 365 * time.Hour) // Last year
			}

			// Set up the mock contract server
			csSettings := contractsmockserver.DefaultSettings()
			csSettings.Token.OnSuccess.Value = azureADToken
			csSettings.Subscription.OnSuccess.Value = proToken
			server := contractsmockserver.NewServer(csSettings)
			err := server.Serve(ctx, "localhost:0")
			require.NoError(t, err, "Setup: Server should return no error")
			//nolint:errcheck // Nothing we can do about it
			defer server.Stop()

			csAddr, err := url.Parse(fmt.Sprintf("http://%s", server.Address()))
			require.NoError(t, err, "Setup: Server URL should have been parsed with no issues")

			err = ubuntupro.FetchFromMicrosoftStore(ctx, conf, nil, contracts.WithProURL(csAddr), contracts.WithMockMicrosoftStore(store))
			if tc.wantErr {
				require.Error(t, err, "FetchFromMicrosoftStore should return an error")
				return
			}
			require.NoError(t, err, "FetchFromMicrosoftStore should return no errors")

			token, _, err := conf.Subscription()
			require.NoError(t, err, "ProToken should return no error")
			require.Equal(t, tc.wantToken, token, "Unexpected value for ProToken")
		})
	}
}

type mockMSStore struct {
	jwt    string
	jwtErr bool

	expirationDate    time.Time
	expirationDateErr bool
}

func (s mockMSStore) GenerateUserJWT(azureADToken string) (jwt string, err error) {
	if s.jwtErr {
		return "", errors.New("mock error")
	}

	return s.jwt, nil
}

func (s mockMSStore) GetSubscriptionExpirationDate() (tm time.Time, err error) {
	if s.expirationDateErr {
		return time.Time{}, errors.New("mock error")
	}

	return s.expirationDate, nil
}

type mockConfig struct {
	storeProToken    string
	registryProToken string

	subscriptionErr     bool
	setStoreProTokenErr bool
}

func (c mockConfig) Subscription() (string, config.Source, error) {
	if c.subscriptionErr {
		return "", config.SourceNone, errors.New("mock config Subscription: mock error")
	}

	if c.registryProToken != "" {
		return c.registryProToken, config.SourceRegistry, nil
	}

	if c.storeProToken != "" {
		return c.storeProToken, config.SourceMicrosoftStore, nil
	}

	return "USER_PRO_TOKEN", config.SourceUser, nil
}

func (c *mockConfig) SetStoreSubscription(ctx context.Context, token string) error {
	if c.setStoreProTokenErr {
		return errors.New("mock config SetStoreSubscription: mock error")
	}

	c.storeProToken = token
	return nil
}
