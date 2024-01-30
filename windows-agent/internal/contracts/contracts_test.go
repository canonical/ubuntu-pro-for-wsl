package contracts_test

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"testing"
	"time"

	"github.com/canonical/ubuntu-pro-for-wsl/mocks/contractserver/contractsmockserver"
	"github.com/canonical/ubuntu-pro-for-wsl/storeapi/go-wrapper/microsoftstore"
	"github.com/canonical/ubuntu-pro-for-wsl/windows-agent/internal/contracts"
	"github.com/stretchr/testify/require"
)

func TestProToken(t *testing.T) {
	t.Parallel()

	//nolint:gosec // These are not real tokens
	const (
		azureADToken   = "AZURE_AD_TOKEN"
		ubuntuProToken = "UBUNTU_PRO_TOKEN"
	)

	testCases := map[string]struct {
		// Microsoft store
		jwtError bool

		// Contract server
		getServerAccessTokenErr bool
		getProTokenErr          bool

		wantErr bool
	}{
		"Success": {},

		"Error when the store's GenerateUserJWT fails":                {jwtError: true, wantErr: true},
		"Error when the contract server's GetServerAccessToken fails": {getServerAccessTokenErr: true, wantErr: true},
		"Error when the contract server's GetProToken fails":          {getProTokenErr: true, wantErr: true},
	}

	for name, tc := range testCases {
		tc := tc
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			ctx := context.Background()
			store := mockMSStore{
				expirationDate: time.Now().Add(24 * 365 * time.Hour), // Next year

				jwt:            "JWT_123",
				jwtWantADToken: azureADToken,
				jwtErr:         tc.jwtError,
			}

			settings := contractsmockserver.DefaultSettings()

			settings.Token.OnSuccess.Value = azureADToken
			settings.Subscription.OnSuccess.Value = ubuntuProToken

			settings.Token.Disabled = tc.getServerAccessTokenErr
			settings.Subscription.Disabled = tc.getProTokenErr

			server := contractsmockserver.NewServer(settings)
			err := server.Serve(ctx, "localhost:0")
			require.NoError(t, err, "Setup: Server should return no error")
			//nolint:errcheck // Nothing we can do about it
			defer server.Stop()

			addr := server.Address()
			url, err := url.Parse(fmt.Sprintf("http://%s", addr))
			require.NoError(t, err, "Setup: Server URL should have been parsed with no issues")

			token, err := contracts.NewProToken(ctx, contracts.WithProURL(url), contracts.WithMockMicrosoftStore(store))
			if tc.wantErr {
				require.Error(t, err, "ProToken should return an error")
				return
			}
			require.NoError(t, err, "ProToken should return no error")

			require.Equal(t, ubuntuProToken, token, "Unexpected value for the pro token")
		})
	}
}

func TestValidSubscription(t *testing.T) {
	t.Parallel()

	type subscriptionStatus int
	const (
		subscribed subscriptionStatus = iota
		expired
		unsubscribed
	)

	testCases := map[string]struct {
		status        subscriptionStatus
		expirationErr bool

		want    bool
		wantErr bool
	}{
		"Succcess when the current subscription is active":  {status: subscribed, want: true},
		"Succcess when the current subscription is expired": {status: expired, want: false},
		"Success when there is no subscription":             {status: unsubscribed, want: false},

		"Error when subscription validity cannot be ascertained": {status: subscribed, expirationErr: true, wantErr: true},
	}

	for name, tc := range testCases {
		tc := tc
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			var store mockMSStore

			switch tc.status {
			case subscribed:
				store.expirationDate = time.Now().Add(time.Hour * 24 * 365) // Next year
			case expired:
				store.expirationDate = time.Now().Add(-time.Hour * 24 * 365) // Last year
			case unsubscribed:
				store.notSubscribed = true
			}

			if tc.expirationErr {
				store.expirationDateErr = true
			}

			got, err := contracts.ValidSubscription(contracts.WithMockMicrosoftStore(store))
			if tc.wantErr {
				require.Error(t, err, "contracts.ValidSubscription should have returned an error")
				return
			}

			require.NoError(t, err, "contracts.ValidSubscription should have returned no error")
			require.Equal(t, tc.want, got, "Unexpected return from ValidSubscription")
		})
	}
}

type mockMSStore struct {
	jwt            string
	jwtWantADToken string
	jwtErr         bool

	notSubscribed     bool
	expirationDate    time.Time
	expirationDateErr bool
}

func (s mockMSStore) GenerateUserJWT(azureADToken string) (jwt string, err error) {
	if s.jwtErr {
		return "", errors.New("mock error")
	}

	if azureADToken != s.jwtWantADToken {
		return "", fmt.Errorf("Azure AD token does not match. Want %q and got %q", s.jwtWantADToken, azureADToken)
	}

	return s.jwt, nil
}

func (s mockMSStore) GetSubscriptionExpirationDate() (tm time.Time, err error) {
	if s.expirationDateErr {
		return time.Time{}, fmt.Errorf("mock error: %w", microsoftstore.ErrStoreAPI)
	}

	if s.notSubscribed {
		return time.Time{}, fmt.Errorf("mock error: %w", microsoftstore.ErrNotSubscribed)
	}

	return s.expirationDate, nil
}
