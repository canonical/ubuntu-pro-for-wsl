// Package client interfaces with the Contracts Server backend.
package client

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"

	"github.com/ubuntu/decorate"
)

// httpDoer is an interface to allow injecting an HTTP Client.
type httpDoer interface {
	Do(*http.Request) (*http.Response, error)
}

// Client knows how to talk to the Contracts Server backend.
type Client struct {
	baseURL *url.URL
	http    httpDoer
}

// NewClient returns a Client instance caching a base URL.
func NewClient(base *url.URL, doer httpDoer) *Client {
	return &Client{
		baseURL: base,
		http:    doer,
	}
}

const (
	apiVersion = "/v1"

	// endpoints.
	tokenPath        = "/token"
	subscriptionPath = "/subscription"

	// A safe token response size - tests with the real MS APIs suggested that those tokens will stay in between 1.2kB to 1.7kB.
	// Our Pro Token is much, much smaller.
	apiTokenMaxSize = 4096

	// JSON keys commonly referred in the Contracts Server backend REST API.
	//nolint:gosec // G101 false positive, this is not a credential
	adTokenKey  = "azure_ad_token"
	jwtKey      = "ms_store_id_key"
	proTokenKey = "contract_token"
)

// GetServerAccessToken returns a short-lived auth token identifying the Contract Server backend.
func (c *Client) GetServerAccessToken(ctx context.Context) (token string, err error) {
	defer decorate.OnError(&err, "couldn't download access token from server")

	// baseurl/v1/token.
	u := c.baseURL.JoinPath(apiVersion, tokenPath)
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
	var data map[string]string
	if err := json.NewDecoder(res.Body).Decode(&data); err != nil {
		return "", fmt.Errorf("failed to decode response body: %v", err)
	}

	val, ok := data[adTokenKey]
	if !ok {
		return "", fmt.Errorf("expected key %q not found in the response", adTokenKey)
	}

	return val, nil
}

// GetProToken returns the (possibly known) Pro Token provided by the Contract Server backend by POST'ing the user JWT.
func (c *Client) GetProToken(ctx context.Context, userJWT string) (token string, err error) {
	defer decorate.OnError(&err, "couldn't download a Pro Token from server")

	if err := checkLength(int64(len(userJWT))); err != nil {
		return "", fmt.Errorf("invalid user JWT: %v", err)
	}

	// baseurl/v1/subscription.
	u := c.baseURL.JoinPath(apiVersion, subscriptionPath)
	jsonData, err := json.Marshal(map[string]string{jwtKey: userJWT})
	if err != nil {
		return "", err
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
	case 401:
		return "", fmt.Errorf("bad user ID key: %v", userJWT)
	case 500:
		return "", errors.New("couldn't validate the user entitlement against MS Store")
	default:
		return "", fmt.Errorf("unknown error from the contracts server response. Code=%d. Body=%s", res.StatusCode, res.Body)
	case 200:
	}

	var data map[string]string
	if err := json.NewDecoder(res.Body).Decode(&data); err != nil {
		return "", err
	}

	val, ok := data[proTokenKey]
	if !ok {
		return "", fmt.Errorf("expected key %q not found in the response", proTokenKey)
	}

	return val, nil
}

// checkLength sanity checks that 0 < length < apiTokenMaxSize.
func checkLength(length int64) error {
	if length < 0 {
		return errors.New("negative length")
	}

	if length == 0 {
		return errors.New("empty")
	}

	if length > apiTokenMaxSize {
		return fmt.Errorf("too big: %d bytes, limit is %d", length, apiTokenMaxSize)
	}

	return nil
}
