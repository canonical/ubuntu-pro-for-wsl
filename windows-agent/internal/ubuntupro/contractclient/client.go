// Package contractclient interfaces with the Contracts Server backend.
package contractclient

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/canonical/ubuntu-pro-for-wsl/common"
	"github.com/canonical/ubuntu-pro-for-wsl/contractsapi"
	"github.com/ubuntu/decorate"
)

// HTTPDoer is an interface to allow injecting an HTTP Client.
type HTTPDoer interface {
	Do(*http.Request) (*http.Response, error)
}

// Client knows how to talk to the Contracts Server backend.
type Client struct {
	baseURL *url.URL
	http    HTTPDoer
}

// New returns a Client instance caching a base URL.
func New(base *url.URL, doer HTTPDoer) *Client {
	return &Client{
		baseURL: base,
		http:    doer,
	}
}

// GetServerAccessToken returns a short-lived auth token identifying the Contract Server backend.
func (c *Client) GetServerAccessToken(ctx context.Context) (token string, err error) {
	defer decorate.OnError(&err, "couldn't download auth token from the contracts server")

	// baseurl/v1/token.
	u := c.baseURL.JoinPath(contractsapi.Version, contractsapi.TokenPath)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return "", fmt.Errorf("could not create a GET request: %v", err)
	}

	req.Header.Set("Accept", "application/json")

	res, err := c.http.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to execute the GET request: %v", err)
	}

	if err := checkLength(res.ContentLength); err != nil {
		return "", fmt.Errorf("invalid response content length: %v", err)
	}

	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		body, err := io.ReadAll(res.Body)
		if err != nil {
			return "", fmt.Errorf("server replied with an error: Code %d, %v", res.StatusCode, err)
		}
		return "", fmt.Errorf("server replied with an error: Code %d, %s", res.StatusCode, body)
	}

	var data map[string]string
	if err := json.NewDecoder(res.Body).Decode(&data); err != nil {
		return "", fmt.Errorf("failed to decode response body: %v", err)
	}

	val, ok := data[contractsapi.ADTokenKey]
	if !ok {
		return "", fmt.Errorf("expected key %q not found in the response", contractsapi.ADTokenKey)
	}

	return val, nil
}

// GetProToken returns the (possibly known) Pro Token provided by the Contract Server backend by POST'ing the user JWT.
func (c *Client) GetProToken(ctx context.Context, userJWT string) (token string, err error) {
	defer decorate.OnError(&err, "couldn't download an Ubuntu Pro Token from the contract server")

	if err := checkLength(int64(len(userJWT))); err != nil {
		return "", fmt.Errorf("invalid user JWT: %v", err)
	}

	// baseurl/v1/subscription.
	u := c.baseURL.JoinPath(contractsapi.Version, contractsapi.SubscriptionPath)

	jsonData, err := json.Marshal(contractsapi.SubscriptionRequest{
		MSStoreIDKey: userJWT,
	})
	if err != nil {
		return "", fmt.Errorf("could not encode the request: %v", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u.String(), bytes.NewReader(jsonData))
	if err != nil {
		return "", fmt.Errorf("could not create a POST request: %v", err)
	}

	res, err := c.http.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to execute the POST request: %v", err)
	}

	if err := checkLength(res.ContentLength); err != nil {
		return "", fmt.Errorf("invalid response content length: %v", err)
	}

	defer res.Body.Close()
	switch res.StatusCode { // add other error codes as CS team documents them.
	case http.StatusUnauthorized:
		return "", fmt.Errorf("bad user ID key: %v", userJWT)
	case http.StatusInternalServerError:
		return "", errors.New("couldn't validate the user entitlement against MS Store")
	case http.StatusOK:
	default:
		body, err := io.ReadAll(res.Body)
		if err != nil {
			return "", fmt.Errorf("unknown error from the contracts server: Code %d, %v", res.StatusCode, err)
		}
		return "", fmt.Errorf("unknown error from the contracts server: Code %d, %s", res.StatusCode, body)
	}

	var resp contractsapi.SyncUserSubscriptionsResponse
	if err := json.NewDecoder(res.Body).Decode(&resp); err != nil {
		return "", fmt.Errorf("could not decode the response: %v", err)
	}

	for product, subscription := range resp.SubscriptionEntitlements {
		if !strings.HasPrefix(product, common.MsStoreProductID) {
			continue
		}

		if subscription.Token == "" {
			// Some other entry may contain the token?
			continue
		}

		return subscription.Token, nil
	}

	return "", fmt.Errorf("response did not contain any valid subscriptions: %s", res.Body)
}

// checkLength sanity checks that 0 < length < apiTokenMaxSize.
func checkLength(length int64) error {
	if length < 0 {
		return errors.New("negative length")
	}

	if length == 0 {
		return errors.New("empty")
	}

	if length > contractsapi.TokenMaxSize {
		return fmt.Errorf("too big: %d bytes, limit is %d", length, contractsapi.TokenMaxSize)
	}

	return nil
}
