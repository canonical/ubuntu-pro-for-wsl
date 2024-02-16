// Package contracts manages Microsoft-Store-entitled subscriptions.
package contracts

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/canonical/ubuntu-pro-for-wsl/storeapi/go-wrapper/microsoftstore"
	"github.com/canonical/ubuntu-pro-for-wsl/windows-agent/internal/contracts/contractclient"
	"github.com/ubuntu/decorate"
)

type options struct {
	proURL         *url.URL
	microsoftStore MicrosoftStore
}

// Option is an optional argument for ProToken.
type Option func(*options)

// WithProURL overrides the Ubuntu Pro contract server URL.
func WithProURL(proURL *url.URL) Option {
	return func(o *options) {
		o.proURL = proURL
	}
}

// WithMockMicrosoftStore overrides the storeAPI-backed Microsoft Store.
func WithMockMicrosoftStore(store MicrosoftStore) Option {
	return func(o *options) {
		o.microsoftStore = store
	}
}

// MicrosoftStore is an interface to the Microsoft store API.
type MicrosoftStore interface {
	GenerateUserJWT(azureADToken string) (jwt string, err error)
	GetSubscriptionExpirationDate() (tm time.Time, err error)
}

// msftStoreDLL is the Microsoft Store backed by the storeapi DLL.
type msftStoreDLL struct{}

func (msftStoreDLL) GenerateUserJWT(azureADToken string) (jwt string, err error) {
	return microsoftstore.GenerateUserJWT(azureADToken)
}

func (msftStoreDLL) GetSubscriptionExpirationDate() (tm time.Time, err error) {
	return microsoftstore.GetSubscriptionExpirationDate()
}

// ValidSubscription returns true if there is a subscription via the Microsoft Store and it is not expired.
func ValidSubscription(args ...Option) (bool, error) {
	opts := options{
		microsoftStore: msftStoreDLL{},
	}

	for _, f := range args {
		f(&opts)
	}

	expiration, err := opts.microsoftStore.GetSubscriptionExpirationDate()
	if err != nil {
		var target microsoftstore.StoreAPIError
		if errors.As(err, &target) && target == microsoftstore.ErrNotSubscribed {
			// ValidSubscription -> false: we are not subscribed
			return false, nil
		}

		return false, err
	}

	if expiration.Before(time.Now()) {
		// ValidSubscription -> false: the subscription is expired
		return false, nil
	}

	// ValidSubscription -> true: the subscription is not yet expired
	return true, nil
}

// NewProToken directs the dance between the Microsoft Store and the Ubuntu Pro contract server to
// validate a store entitlement and obtain its associated pro token. If there is no entitlement,
// the token is returned as an empty string.
func NewProToken(ctx context.Context, args ...Option) (token string, err error) {
	defer decorate.OnError(&err, "couldn't get a Microsoft-Store-provided Ubuntu Pro token")

	opts := options{
		microsoftStore: msftStoreDLL{},
	}

	for _, f := range args {
		f(&opts)
	}

	if opts.proURL == nil {
		url, err := defaultProBackendURL()
		if err != nil {
			return "", fmt.Errorf("could not parse contract server URL: %v", err)
		}
		opts.proURL = url
	}

	contractClient := contractclient.New(opts.proURL, &http.Client{Timeout: 30 * time.Second})
	msftStore := opts.microsoftStore

	adToken, err := contractClient.GetServerAccessToken(ctx)
	if err != nil {
		return "", err
	}

	storeToken, err := msftStore.GenerateUserJWT(adToken)
	if err != nil {
		return "", err
	}

	proToken, err := contractClient.GetProToken(ctx, storeToken)
	if err != nil {
		return "", err
	}

	return proToken, nil
}
