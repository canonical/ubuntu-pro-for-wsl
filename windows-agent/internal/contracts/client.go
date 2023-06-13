// Package contracts interfaces with the Contracts Server backend.
package contracts

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
func (c *Client) GetServerAccessToken(ctx context.Context) (t string, err error) {
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
		return "", fmt.Errorf("failed to exeecute the GET request: %v", err)
	}

	if err := checkContentLength(res.ContentLength); err != nil {
		return "", err
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
func (c *Client) GetProToken(ctx context.Context, userJwt string) (t string, err error) {
	defer decorate.OnError(&err, "couldn't download a Pro Token from server")

	jwtLen := len(userJwt)
	if jwtLen == 0 {
		return "", errors.New("user JWT cannot be empty")
	}

	if jwtLen > apiTokenMaxSize {
		return "", errors.New("too big JWT")
	}

	// baseurl/v1/subscription.
	u := c.baseURL.JoinPath(apiVersion, subscriptionPath)
	jsonData, err := json.Marshal(map[string]string{jwtKey: userJwt})
	if err != nil {
		return "", err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u.String(), bytes.NewReader(jsonData))
	if err != nil {
		return "", err
	}

	res, err := c.http.Do(req)
	if err != nil {
		return "", err
	}

	if err := checkContentLength(res.ContentLength); err != nil {
		return "", err
	}

	defer res.Body.Close()
	switch res.StatusCode { // add other error codes as CS team documents them.
	case 401:
		return "", fmt.Errorf("bad user ID key: %v", userJwt)
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

// checkContentLength sanity checks to make sure the decoder won't blow up with strange responses.
func checkContentLength(cl int64) error {
	if cl == -1 {
		return errors.New("cannot accept response of unknown content length")
	}

	if cl == 0 {
		return errors.New("unexpected empty response")
	}

	if cl > apiTokenMaxSize {
		return fmt.Errorf("response is too big: %d bytes", cl)
	}

	return nil
}
