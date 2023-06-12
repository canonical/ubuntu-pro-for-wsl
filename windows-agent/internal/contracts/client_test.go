package contracts_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/canonical/ubuntu-pro-for-windows/windows-agent/internal/contracts"
	"github.com/stretchr/testify/require"
)

func TestGetServerAccessToken(t *testing.T) {
	t.Parallel()

	testCases := map[string]struct {
		responseKey           string
		responseValue         string
		responseCode          int
		responseLengthUnknown bool
		emptyBody             bool
		errorOnDo             bool
		invalidJSON           bool

		wantErr bool
	}{
		"Success": {responseValue: strings.Repeat("Token", 256), responseCode: 200},

		"Error with a too big token":                 {responseValue: strings.Repeat("Token", 1000), responseCode: 200, wantErr: true},
		"Error with empty response":                  {responseCode: 200, emptyBody: true, wantErr: true},
		"Error with unknown content length response": {responseValue: "unbounded", responseCode: 200, responseLengthUnknown: true, wantErr: true},
		"Error with expected key not in response":    {responseKey: "another_token", responseValue: "good", responseCode: 200, wantErr: true},
		"Error on http.Do":                           {errorOnDo: true, wantErr: true},
		"Error with invalid JSON":                    {responseKey: "another_token", responseValue: "good", invalidJSON: true, wantErr: true},
	}

	for name, tc := range testCases {
		tc := tc

		t.Run(name, func(t *testing.T) {
			t.Parallel()

			if tc.responseKey == "" {
				tc.responseKey = contracts.JSONKeyAdToken
			}

			h := HTTPMock{
				errorOnDo:            tc.errorOnDo,
				emptyBody:            tc.emptyBody,
				invalidJSON:          tc.invalidJSON,
				key:                  tc.responseKey,
				value:                tc.responseValue,
				statusCode:           tc.responseCode,
				unknownContentLength: tc.responseLengthUnknown,
			}
			u, err := url.Parse("https://localhost.org")
			require.NoError(t, err, "Setup: URL parsing should not fail")

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
		errorOnDo     bool
		invalidJSON   bool

		wantErr bool
	}{
		"Success": {jwt: "JWT", responseValue: strings.Repeat("Token", 256), responseCode: 200},

		"Error with a too big jwt":                {jwt: strings.Repeat("USER_JWT", 550), wantErr: true},
		"Error with empty jwt":                    {jwt: "", wantErr: true},
		"Error with bad JWT":                      {jwt: "bad", responseValue: "BAD REQUEST", responseCode: 401, wantErr: true},
		"Error with MS API failure":               {jwt: "good", responseValue: "UNKNOWN SERVER ERROR", responseCode: 500, wantErr: true},
		"Error with expected key not in response": {jwt: "good", responseKey: "another_token", responseValue: "good", responseCode: 200, wantErr: true},
		"Error on http.Do":                        {errorOnDo: true, wantErr: true},
	}

	for name, tc := range testCases {
		tc := tc

		t.Run(name, func(t *testing.T) {
			t.Parallel()

			if tc.responseKey == "" {
				tc.responseKey = contracts.JSONKeyProToken
			}

			h := HTTPMock{
				errorOnDo:   tc.errorOnDo,
				emptyBody:   tc.emptyBody,
				invalidJSON: tc.invalidJSON,
				key:         tc.responseKey,
				value:       tc.responseValue,
				statusCode:  tc.responseCode,
			}
			u, err := url.Parse("https://localhost.org")
			require.NoError(t, err, "Setup: URL parsing should not fail")

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

type HTTPMock struct {
	errorOnDo            bool
	emptyBody            bool
	invalidJSON          bool
	unknownContentLength bool
	key                  string
	value                string
	statusCode           int
}

func (m HTTPMock) Do(*http.Request) (*http.Response, error) {
	if m.errorOnDo {
		// desired error. Unrelated to non-2xx status codes.
		return nil, errors.New("Wanted error")
	}

	if m.emptyBody {
		// empty body response.
		return &http.Response{}, nil
	}

	var b []byte
	var err error
	if m.invalidJSON {
		b = []byte(m.key + m.value)
	} else {
		b, err = json.Marshal(map[string]string{m.key: m.value})
	}

	if err != nil {
		return nil, err
	}

	cl := int64(-1)
	if !m.unknownContentLength {
		cl = int64(len(b))
	}

	response := http.Response{
		Body:          io.NopCloser(bytes.NewBuffer(b)),
		StatusCode:    m.statusCode,
		ContentLength: cl,
	}

	return &response, nil
}
