// Package contractsmockserver implements a mocked version of the Contracts Server backend.
// DO NOT USE IN PRODUCTION

package contractsmockserver

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"

	"github.com/canonical/ubuntu-pro-for-windows/windows-agent/internal/contracts/apidef"
)

const (
	DefaultADToken  = "eHy_ADToken"
	DefaultProToken = "CHx_ProToken"
)

type response struct {
	value      string
	statusCode int
}

type options struct {
	token        response
	subscription response
}

type Option func(*options)

// WithTokenResponse sets the value of the /token endpoint response.
func WithTokenResponse(token string) Option {
	return func(o *options) {
		o.token.value = token
	}
}

// WithTokenStatusCode sets the /token endpoint response status code.
func WithTokenStatusCode(statusCode int) Option {
	return func(o *options) {
		o.token.statusCode = statusCode
	}
}

// WithProTokenResponse sets the value of the /subscription endpoint response.
func WithSubscriptionResponse(token string) Option {
	return func(o *options) {
		o.subscription.value = token
	}
}

// WithSubscriptionStatusCode sets the /subscription endpoint response status code.
func WithSubscriptionStatusCode(statusCode int) Option {
	return func(o *options) {
		o.subscription.statusCode = statusCode
	}
}

// Serve starts a new HTTP server on localhost (dynamic port) mocking the Contracts Server backend REST API with responses defined according to the Option args.
func Serve(ctx context.Context, args ...Option) (addr string, err error) {
	opts := options{
		token:        response{value: DefaultADToken, statusCode: http.StatusOK},
		subscription: response{value: DefaultProToken, statusCode: http.StatusOK},
	}

	for _, f := range args {
		f(&opts)
	}

	var lc net.ListenConfig
	lis, err := lc.Listen(ctx, "tcp", "localhost:")
	if err != nil {
		return "", fmt.Errorf("failed to listen over tcp: %v", err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc(apidef.Version+apidef.TokenPath, handleTokenFunc(opts.token))

	go http.Serve(lis, mux)

	return lis.Addr().String(), nil

}

// handleTokenFunc returns a function to handle requests to the /token endpoint according to the response options supplied.
func handleTokenFunc(res response) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			fmt.Fprintln(w, "this endpoint only supports GET")
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		if res.statusCode != 200 {
			fmt.Fprintf(w, `mock error`)
			w.WriteHeader(res.statusCode)
			return
		}

		if _, err := fmt.Fprintf(w, fmt.Sprintf(`{%q: %q}`, apidef.ADTokenKey, res.value)); err != nil {
			fmt.Fprintf(w, "failed to write the response: %v", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusOK)
	}
}
