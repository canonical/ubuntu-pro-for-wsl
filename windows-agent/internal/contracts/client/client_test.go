package client_test

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
	"testing"

	"github.com/canonical/ubuntu-pro-for-windows/windows-agent/internal/contracts/client"
	"github.com/stretchr/testify/require"
)

func TestGetServerAccessToken(t *testing.T) {
	t.Parallel()

	goodToken := strings.Repeat("Token", 256)

	testCases := map[string]struct {
		errorOnDo            bool
		responseContents     []byte
		unknownContentLength bool
		statusCode           int
		nilContext           bool

		wantErr bool
	}{
		"Success": {},

		"Error with a too big token":                 {responseContents: []byte(fmt.Sprintf("{%q:%q}", client.JSONKeyAdToken, strings.Repeat("REPEAT_TOO_BIG_TOKEN", 220))), wantErr: true},
		"Error with empty response":                  {responseContents: []byte(""), wantErr: true},
		"Error with unknown content length response": {unknownContentLength: true, wantErr: true},
		"Error with expected key not in response":    {responseContents: []byte(`{"unexpected_key": "unexpected_value"}`), wantErr: true},
		"Error on http.Do":                           {errorOnDo: true, wantErr: true},
		"Error with invalid JSON":                    {responseContents: []byte("invalid JSON"), wantErr: true},
		"Error with a nil context":                   {nilContext: true, wantErr: true},
	}

	for name, tc := range testCases {
		tc := tc

		t.Run(name, func(t *testing.T) {
			t.Parallel()

			if tc.responseContents == nil {
				var err error
				tc.responseContents, err = json.Marshal(map[string]string{client.JSONKeyAdToken: goodToken})
				require.NoError(t, err, "Setup: unexpected error when marshalling the test token")
			}

			l := int64(len(tc.responseContents))
			if tc.unknownContentLength {
				l = -1
			}
			h := HTTPMock{
				errorOnDo: tc.errorOnDo,
				response:  http.Response{Body: io.NopCloser(bytes.NewReader(tc.responseContents)), StatusCode: tc.statusCode, ContentLength: l},
			}
			u, err := url.Parse("https://localhost:1234")
			require.NoError(t, err, "Setup: URL parsing should not fail")

			client := client.NewClient(u, h)
			ctx := context.Background()
			if tc.nilContext {
				ctx = nil
			}

			got, err := client.GetServerAccessToken(ctx)
			if tc.wantErr {
				require.Errorf(t, err, "Got token %q when failure was expected", got)
				return
			}

			require.NoError(t, err, "GetServerAccessToken should return no errors")

			require.Equal(t, goodToken, got)
		})
	}
}

func TestGetProToken(t *testing.T) {
	t.Parallel()

	goodToken := strings.Repeat("Token", 256)

	testCases := map[string]struct {
		jwt string

		errorOnDo            bool
		responseContents     []byte
		unknownContentLength bool
		statusCode           int
		nilContext           bool

		wantErr bool
	}{
		"Success": {jwt: "JWT"},

		"Error with a too big jwt":                {jwt: strings.Repeat("REPEAT_TOO_BIG_JWT", 230), wantErr: true},
		"Error with empty jwt":                    {jwt: "", wantErr: true},
		"Error with bad request":                  {statusCode: 401, jwt: "bad", responseContents: []byte("BAD REQUEST"), wantErr: true}, // that would mean a JWT the server found to be invalid.
		"Error with MS API failure":               {statusCode: 500, jwt: "JWT", responseContents: []byte("UNKNOWN SERVER ERROR"), wantErr: true},
		"Error with expected key not in response": {responseContents: []byte(`{"unexpected_key": "unexpected_value"}`), jwt: "JWT", wantErr: true},
		"Error on http.Do":                        {errorOnDo: true, jwt: "JWT", wantErr: true},
		"Error with invalid JSON":                 {responseContents: []byte("invalid JSON"), jwt: "JWT", wantErr: true},
		"Error with unexpected status code":       {statusCode: 422, jwt: "JWT", wantErr: true},
		"Error with empty response body":          {responseContents: []byte(""), jwt: "JWT", wantErr: true},
		"Error with unknown response length":      {unknownContentLength: true, jwt: "JWT", wantErr: true},
		"Error with a nil context":                {nilContext: true, jwt: "JWT", wantErr: true},
	}

	for name, tc := range testCases {
		tc := tc

		t.Run(name, func(t *testing.T) {
			t.Parallel()

			if tc.statusCode == 0 {
				tc.statusCode = 200
			}

			if tc.responseContents == nil {
				var err error
				tc.responseContents, err = json.Marshal(map[string]string{client.JSONKeyProToken: goodToken})
				require.NoError(t, err, "Setup: unexpected error when marshalling the good token")
			}

			l := int64(len(tc.responseContents))
			if tc.unknownContentLength {
				l = -1
			}
			h := HTTPMock{
				errorOnDo: tc.errorOnDo,
				response:  http.Response{Body: io.NopCloser(bytes.NewReader(tc.responseContents)), StatusCode: tc.statusCode, ContentLength: l},
			}
			u, err := url.Parse("https://localhost:1234")
			require.NoError(t, err, "Setup: URL parsing should not fail")

			client := client.NewClient(u, h)
			ctx := context.Background()
			if tc.nilContext {
				ctx = nil
			}

			got, err := client.GetProToken(ctx, tc.jwt)
			if tc.wantErr {
				require.Errorf(t, err, "Got token %q when failure was expected", got)
				return
			}

			require.NoError(t, err, "GetProToken should return no errors")

			require.Equal(t, goodToken, got)
		})
	}
}

type HTTPMock struct {
	errorOnDo bool
	response  http.Response
}

func (m HTTPMock) Do(*http.Request) (*http.Response, error) {
	if m.errorOnDo {
		// desired error. Unrelated to non-2xx status codes.
		return nil, errors.New("Wanted error")
	}

	return &m.response, nil
}
