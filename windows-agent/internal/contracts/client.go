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
)

// httpDoer is an interface to allow injecting an HTTP Client.
type httpDoer interface {
	Do(*http.Request) (*http.Response, error)
}

// Client knows how to talk to the Contracts Server backend.
type Client struct {
	baseURL url.URL
	http    httpDoer
}

// NewClient returns a Client instance caching a base URL.
func NewClient(base *url.URL, doer httpDoer) *Client {
	return &Client{
		baseURL: *base,
		http:    doer,
	}
}

// sanity checks to make sure the decoder won't blow up with strange responses.
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

// GetServerAccessToken returns a short-lived auth token identifying the Contract Server backend.
func (c *Client) GetServerAccessToken(ctx context.Context) (string, error) {
	// baseurl/v1/token.
	u := c.baseURL.JoinPath(apiVersion, getToken)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return "", err
	}

	req.Header.Set("Accept", "application/json")

	res, err := c.http.Do(req)
	if err != nil {
		return "", err
	}

	if err := checkContentLength(res.ContentLength); err != nil {
		return "", err
	}

	defer res.Body.Close()
	var data map[string]string
	if err := json.NewDecoder(res.Body).Decode(&data); err != nil {
		return "", err
	}

	if val, ok := data[jsonKeyAdToken]; ok {
		return val, nil
	}

	return "", fmt.Errorf("expected key \"%s\" not found in the response", jsonKeyAdToken)
}

// GetProToken returns the (possibly known) Pro Token provided by the Contract Server backend by POST'ing the user JWT.
func (c *Client) GetProToken(ctx context.Context, userJwt string) (string, error) {
	jwtLen := len(userJwt)
	if jwtLen == 0 {
		return "", errors.New("user JWT cannot be empty")
	}

	if jwtLen > apiTokenMaxSize {
		return "", errors.New("too big JWT")
	}

	// baseurl/v1/subscription.
	u := c.baseURL.JoinPath(apiVersion, postSubscription)
	jsonData, err := json.Marshal(map[string]string{jsonKeyJwt: userJwt})
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

	switch res.StatusCode { // add other error codes as CS team documents them.
	case 401:
		return "", fmt.Errorf("bad user ID key: %v", userJwt)
	case 500:
		return "", errors.New("couldn't validate the user entitlement against MS Store")
	}

	defer res.Body.Close()
	var data map[string]string
	if err := json.NewDecoder(res.Body).Decode(&data); err != nil {
		return "", err
	}

	if val, ok := data[jsonKeyProToken]; ok {
		return val, nil
	}

	return "", fmt.Errorf("expected key \"%s\" not found in the response", jsonKeyProToken)
}
