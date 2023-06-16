// Package client interfaces with the Contracts Server backend.
package client

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"

	"github.com/canonical/ubuntu-pro-for-windows/windows-agent/internal/contracts/apidef"
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

// New returns a Client instance caching a base URL.
func New(base *url.URL, doer httpDoer) *Client {
	return &Client{
		baseURL: base,
		http:    doer,
	}
}

// GetServerAccessToken returns a short-lived auth token identifying the Contract Server backend.
func (c *Client) GetServerAccessToken(ctx context.Context) (token string, err error) {
	defer decorate.OnError(&err, "couldn't download access token from server")

	// baseurl/v1/token.
	u := c.baseURL.JoinPath(apidef.Version, apidef.TokenPath)
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
	if res.StatusCode != 200 {
		bodyBytes, err := io.ReadAll(res.Body)
		if err != nil {
			return "", fmt.Errorf("server replied with an error: Code %d, %v", res.StatusCode, err)
		}
		return "", fmt.Errorf("server replied with an error: Code %d, %s", res.StatusCode, bodyBytes)
	}

	var data map[string]string
	if err := json.NewDecoder(res.Body).Decode(&data); err != nil {
		return "", fmt.Errorf("failed to decode response body: %v", err)
	}

	val, ok := data[apidef.ADTokenKey]
	if !ok {
		return "", fmt.Errorf("expected key %q not found in the response", apidef.ADTokenKey)
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
	u := c.baseURL.JoinPath(apidef.Version, apidef.SubscriptionPath)
	jsonData, err := json.Marshal(map[string]string{apidef.JWTKey: userJWT})
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
		bodyBytes, err := io.ReadAll(res.Body)
		if err != nil {
			return "", fmt.Errorf("unknown error from the contracts server: Code %d, %v", res.StatusCode, err)
		}
		return "", fmt.Errorf("unknown error from the contracts server: Code %d, %s", res.StatusCode, bodyBytes)
	case 200:
	}

	var data map[string]string
	if err := json.NewDecoder(res.Body).Decode(&data); err != nil {
		return "", err
	}

	val, ok := data[apidef.ProTokenKey]
	if !ok {
		return "", fmt.Errorf("expected key %q not found in the response", apidef.ProTokenKey)
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

	if length > apidef.TokenMaxSize {
		return fmt.Errorf("too big: %d bytes, limit is %d", length, apidef.TokenMaxSize)
	}

	return nil
}
