package contracts_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/canonical/ubuntu-pro-for-windows/windows-agent/internal/contracts"
	"github.com/stretchr/testify/require"
)

type HTTPMock struct {
	EmptyBody            bool
	UnknownContentLength bool
	Key                  string
	Value                string
	StatusCode           int
}

func (m HTTPMock) Do(*http.Request) (*http.Response, error) {
	if m.EmptyBody {
		// empty body response.
		return &http.Response{}, nil
	}

	b, err := json.Marshal(map[string]string{m.Key: m.Value})
	if err != nil {
		return nil, err
	}

	cl := int64(-1)
	if !m.UnknownContentLength {
		cl = int64(len(b))
	}

	response := http.Response{
		Body:          io.NopCloser(bytes.NewBuffer(b)),
		StatusCode:    m.StatusCode,
		ContentLength: cl,
	}

	return &response, nil
}

func TestGetServerAccessToken(t *testing.T) {
	t.Parallel()

	testCases := map[string]struct {
		responseKey           string
		responseValue         string
		responseCode          int
		responseLengthUnknown bool

		emptyBody bool
		wantErr   bool
	}{
		"Success": {responseValue: strings.Repeat("Token", 256), responseCode: 200},

		"Fail with a too big token":                 {responseValue: strings.Repeat("Token", 1000), responseCode: 200, wantErr: true},
		"Fail with empty response":                  {responseCode: 200, emptyBody: true, wantErr: true},
		"Fail with unknown content length response": {responseValue: "unbounded", responseCode: 200, responseLengthUnknown: true, wantErr: true},
		"Fail with expected key not in response":    {responseKey: "another_token", responseValue: "good", responseCode: 200, wantErr: true},
	}

	for name, tc := range testCases {
		tc := tc

		t.Run(name, func(t *testing.T) {
			t.Parallel()

			if tc.responseKey == "" {
				tc.responseKey = contracts.JSONKeyAdToken
			}

			h := HTTPMock{
				EmptyBody:            tc.emptyBody,
				Key:                  tc.responseKey,
				Value:                tc.responseValue,
				StatusCode:           tc.responseCode,
				UnknownContentLength: tc.responseLengthUnknown,
			}
			u, _ := url.Parse("https://localhost.org")
			client := contracts.NewClient(u, h)

			aad, err := client.GetServerAccessToken(context.Background())

			if tc.wantErr {
				require.Errorf(t, err, "Got token %q when failure was expected", aad)
				return
			}

			require.NoError(t, err, "GetServerAccessToken should return no errors")
		})
	}
}

func TestGetProToken(t *testing.T) {
	t.Parallel()

	testCases := map[string]struct {
		jwt string

		responseKey   string
		responseValue string
		responseCode  int
		emptyBody     bool

		wantErr bool
	}{
		"Success": {jwt: "JWT", responseValue: strings.Repeat("Token", 256), responseCode: 200},

		"Fail with a too big jwt":                {jwt: strings.Repeat("USER_JWT", 550), wantErr: true},
		"Fail with empty jwt":                    {jwt: "", wantErr: true},
		"Fail with bad JWT":                      {jwt: "bad", responseValue: "BAD REQUEST", responseCode: 401, wantErr: true},
		"Fail with MS API failure":               {jwt: "good", responseValue: "UNKNOWN SERVER ERROR", responseCode: 500, wantErr: true},
		"Fail with expected key not in response": {jwt: "good", responseKey: "another_token", responseValue: "good", responseCode: 200, wantErr: true},
	}

	for name, tc := range testCases {
		tc := tc

		t.Run(name, func(t *testing.T) {
			t.Parallel()

			if tc.responseKey == "" {
				tc.responseKey = contracts.JSONKeyProToken
			}

			h := HTTPMock{
				EmptyBody:  tc.emptyBody,
				Key:        tc.responseKey,
				Value:      tc.responseValue,
				StatusCode: tc.responseCode,
			}
			u, _ := url.Parse("https://localhost.org")
			client := contracts.NewClient(u, h)

			proToken, err := client.GetProToken(context.Background(), tc.jwt)

			if tc.wantErr {
				require.Errorf(t, err, "Got token %q when failure was expected", proToken)
				return
			}

			require.NoError(t, err, "GetProToken should return no errors")
		})
	}
}
