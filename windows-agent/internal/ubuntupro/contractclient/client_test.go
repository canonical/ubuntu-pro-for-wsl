package contractclient_test

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
	"time"

	"github.com/canonical/ubuntu-pro-for-wsl/common"
	"github.com/canonical/ubuntu-pro-for-wsl/contractsapi"
	"github.com/canonical/ubuntu-pro-for-wsl/mocks/contractserver/contractsmockserver"
	"github.com/canonical/ubuntu-pro-for-wsl/windows-agent/internal/ubuntupro/contractclient"
	"github.com/stretchr/testify/require"
)

func TestGetServerAccessToken(t *testing.T) {
	t.Parallel()

	goodToken := strings.Repeat("Token", 256)

	testCases := map[string]struct {
		errorOnDo            bool
		responseContent      []byte
		unknownContentLength bool
		statusCode           int
		nilContext           bool

		want    string
		wantErr bool
	}{
		"Success": {want: goodToken},

		"Error with a too big token":                 {responseContent: []byte(fmt.Sprintf("{%q:%q}", contractsapi.ADTokenKey, strings.Repeat("REPEAT_TOO_BIG_TOKEN", 220))), wantErr: true},
		"Error with empty response":                  {responseContent: []byte(""), wantErr: true},
		"Error with unknown content length response": {unknownContentLength: true, wantErr: true},
		"Error with expected key not in response":    {responseContent: []byte(`{"unexpected_key": "unexpected_value"}`), wantErr: true},
		"Error on http.Do":                           {errorOnDo: true, wantErr: true},
		"Error with invalid JSON":                    {responseContent: []byte("invalid JSON"), wantErr: true},
		"Error with a nil context":                   {nilContext: true, wantErr: true},
	}

	for name, tc := range testCases {

		t.Run(name, func(t *testing.T) {
			t.Parallel()

			if tc.statusCode == 0 {
				tc.statusCode = http.StatusOK
			}

			if tc.responseContent == nil {
				var err error
				tc.responseContent, err = json.Marshal(map[string]string{contractsapi.ADTokenKey: goodToken})
				require.NoError(t, err, "Setup: unexpected error when marshalling the test token")
			}

			l := int64(len(tc.responseContent))
			if tc.unknownContentLength {
				l = -1
			}
			h := HTTPMock{
				errorOnDo: tc.errorOnDo,
				response:  http.Response{Body: io.NopCloser(bytes.NewReader(tc.responseContent)), StatusCode: tc.statusCode, ContentLength: l},
			}
			u, err := url.Parse("https://localhost:1234")
			require.NoError(t, err, "Setup: URL parsing should not fail")

			client := contractclient.New(u, h)
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

			require.Equal(t, tc.want, got)
		})
	}
}

func TestGetProToken(t *testing.T) {
	t.Parallel()

	goodToken := strings.Repeat("Token", 256)

	responseWithToken := func(t *testing.T, token string) []byte {
		t.Helper()

		r, err := json.Marshal(contractsapi.SyncUserSubscriptionsResponse{
			SubscriptionEntitlements: map[string]contractsapi.SyncUserSubscriptionsResponseItem{
				common.MsStoreProductID + ":0001": {Token: token},
				"SOME_OTHER_PRODUCT:0002":         {Token: "token-to-ignore"},
			},
		})

		require.NoError(t, err, "Setup: unexpected error when marshalling the good token")
		return r
	}

	testCases := map[string]struct {
		jwt string

		errorOnDo            bool
		responseContent      []byte
		unknownContentLength bool
		statusCode           int
		nilContext           bool

		want    string
		wantErr bool
	}{
		"Success": {jwt: "JWT", want: goodToken},

		"Error with a too big jwt":                {jwt: strings.Repeat("REPEAT_TOO_BIG_JWT", 230), wantErr: true},
		"Error with empty jwt":                    {jwt: "-", wantErr: true},
		"Error with bad request":                  {statusCode: 401, jwt: "bad JWT", responseContent: []byte("BAD REQUEST"), wantErr: true}, // that would mean a JWT the server found to be invalid.
		"Error with MS API failure":               {statusCode: 500, responseContent: []byte("UNKNOWN SERVER ERROR"), wantErr: true},
		"Error with expected key not in response": {responseContent: []byte(`{"unexpected_key": "unexpected_value"}`), wantErr: true},
		"Error on http.Do":                        {errorOnDo: true, wantErr: true},
		"Error with invalid JSON":                 {responseContent: []byte("invalid JSON"), wantErr: true},
		"Error with empty token":                  {responseContent: responseWithToken(t, ""), wantErr: true},
		"Error with unexpected status code":       {statusCode: 422, wantErr: true},
		"Error with empty response body":          {responseContent: []byte(""), wantErr: true},
		"Error with unknown response length":      {unknownContentLength: true, wantErr: true},
		"Error with a nil context":                {nilContext: true, wantErr: true},
	}

	for name, tc := range testCases {

		t.Run(name, func(t *testing.T) {
			t.Parallel()

			if len(tc.jwt) == 0 { // we want a simple default.
				tc.jwt = "JWT"
			} else if tc.jwt == "-" { // we want to exercise the case of the empty string.
				tc.jwt = ""
			}

			if tc.statusCode == 0 {
				tc.statusCode = http.StatusOK
			}

			if tc.responseContent == nil {
				tc.responseContent = responseWithToken(t, goodToken)
			}

			l := int64(len(tc.responseContent))
			if tc.unknownContentLength {
				l = -1
			}
			h := HTTPMock{
				errorOnDo: tc.errorOnDo,
				response:  http.Response{Body: io.NopCloser(bytes.NewReader(tc.responseContent)), StatusCode: tc.statusCode, ContentLength: l},
			}
			u, err := url.Parse("https://localhost:1234")
			require.NoError(t, err, "Setup: URL parsing should not fail")

			client := contractclient.New(u, h)
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

			require.Equal(t, tc.want, got)
		})
	}
}

func TestGetServerAccessTokenNet(t *testing.T) {
	t.Parallel()

	testCases := map[string]struct {
		dontServe        bool
		preCancel        bool
		withToken        string
		withStatus       int
		disabledEndpoint bool
		blockedEndpoint  bool

		want    string
		wantErr bool
	}{
		"Success": {want: contractsmockserver.DefaultADToken},

		"Error due to no server":               {dontServe: true, wantErr: true},
		"Error due to precanceled context":     {preCancel: true, wantErr: true},
		"Error due to non-200 status code":     {withStatus: 418, wantErr: true},
		"Error due to disabled endpoint (404)": {disabledEndpoint: true, wantErr: true},
		"Error due to response timeout":        {blockedEndpoint: true, wantErr: true},
	}

	for name, tc := range testCases {

		t.Run(name, func(t *testing.T) {
			t.Parallel()

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			addr := "localhost:9" // IANA Discard Protocol.
			var err error

			if !tc.dontServe {
				settings := contractsmockserver.DefaultSettings()

				if len(tc.withToken) > 0 {
					settings.Token.OnSuccess.Value = tc.withToken
				}

				if tc.withStatus != 0 && tc.withStatus != 200 {
					settings.Token.OnSuccess.Status = tc.withStatus
				}

				settings.Token.Disabled = tc.disabledEndpoint
				settings.Token.Blocked = tc.blockedEndpoint

				s := contractsmockserver.NewServer(settings)

				err := s.Serve(ctx, "localhost:0")
				require.NoError(t, err, "Setup: Server should return no error")

				addr = s.Address()
				//nolint:errcheck // Nothing we can do about it
				defer s.Stop()
			}

			u, err := url.Parse(fmt.Sprintf("http://%s", addr))
			require.NoError(t, err, "Setup: URL parsing should not fail")

			client := contractclient.New(u, &http.Client{Timeout: 3 * time.Second})

			clientCtx, clientCancel := context.WithCancel(ctx)
			if tc.preCancel {
				clientCancel()
			}
			defer clientCancel()

			got, err := client.GetServerAccessToken(clientCtx)
			if tc.wantErr {
				require.Errorf(t, err, "Got token %q when failure was expected", got)
				return
			}
			require.NoError(t, err, "GetServerAccessToken should return no errors")

			require.Equal(t, tc.want, got, "GetServerAccessToken's return value does not match the expected one")
		})
	}
}

func TestGetProTokenNet(t *testing.T) {
	t.Parallel()

	testCases := map[string]struct {
		dontServe        bool
		preCancel        bool
		withToken        string
		withStatus       int
		disabledEndpoint bool
		blockedEndpoint  bool

		want    string
		wantErr bool
	}{
		"Success": {want: contractsmockserver.DefaultProToken},

		"Error due to no server":               {dontServe: true, wantErr: true},
		"Error due to precanceled context":     {preCancel: true, wantErr: true},
		"Error due to non-200 status code":     {withStatus: 418, wantErr: true},
		"Error due to disabled endpoint (404)": {disabledEndpoint: true, wantErr: true},
		"Error due to response timeout":        {blockedEndpoint: true, wantErr: true},
	}

	for name, tc := range testCases {

		t.Run(name, func(t *testing.T) {
			t.Parallel()

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			addr := "localhost:9" // IANA Discard Protocol.
			var err error

			if !tc.dontServe {
				settings := contractsmockserver.DefaultSettings()

				if len(tc.withToken) > 0 {
					settings.Subscription.OnSuccess.Value = tc.withToken
				}

				if tc.withStatus != 0 && tc.withStatus != 200 {
					settings.Subscription.OnSuccess.Status = tc.withStatus
				}

				settings.Subscription.Disabled = tc.disabledEndpoint
				settings.Subscription.Blocked = tc.blockedEndpoint

				s := contractsmockserver.NewServer(settings)

				err = s.Serve(ctx, "localhost:0")
				require.NoError(t, err, "Setup: Server should return no error")

				addr = s.Address()
				//nolint:errcheck // Nothing we can do about it
				defer s.Stop()
			}

			u, err := url.Parse(fmt.Sprintf("http://%s", addr))
			require.NoError(t, err, "Setup: URL parsing should not fail")

			client := contractclient.New(u, &http.Client{Timeout: 3 * time.Second})

			clientCtx, clientCancel := context.WithCancel(ctx)
			if tc.preCancel {
				clientCancel()
			}
			defer clientCancel()

			got, err := client.GetProToken(clientCtx, "JWT")
			if tc.wantErr {
				require.Errorf(t, err, "Got token %q when failure was expected", got)
				return
			}
			require.NoError(t, err, "GetProToken should return no errors")

			require.Equal(t, tc.want, got, "GetProToken's return value does not match the expected one")
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
