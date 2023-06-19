// Package contractsmockserver implements a mocked version of the Contracts Server backend.
// DO NOT USE IN PRODUCTION
package contractsmockserver

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"time"

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

// Option is an optional argument for Serve.
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

// WithSubscriptionResponse sets the value of the /subscription endpoint response.
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
	mux.HandleFunc(apidef.Version+apidef.SubscriptionPath, handleSubscriptionFunc(opts.subscription))

	go func() {
		server := &http.Server{
			Addr:              addr,
			Handler:           mux,
			ReadHeaderTimeout: 3 * time.Second,
		}
		if err := server.Serve(lis); err != nil {
			fmt.Println("failed to start the HTTP server")
		}
	}()

	return lis.Addr().String(), nil
}

// handleTokenFunc returns a a handler function for the /token endpoint according to the response options supplied.
func handleTokenFunc(res response) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			fmt.Fprintln(w, "this endpoint only supports GET")
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		if res.statusCode != 200 {
			fmt.Fprintf(w, "mock error")
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

// handleSubscriptionFunc returns a handler function for the /susbcription endpoint according to the response options supplied.
func handleSubscriptionFunc(res response) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			fmt.Fprintln(w, "this endpoint only supports POST")
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		if res.statusCode != 200 {
			fmt.Fprintf(w, "mock error")
			w.WriteHeader(res.statusCode)
			return
		}

		var data map[string]string
		if err := json.NewDecoder(r.Body).Decode(&data); err != nil {
			fmt.Fprintln(w, "Bad Request")
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		userJWT, ok := data[apidef.JWTKey]
		if !ok {
			fmt.Fprintln(w, "JSON payload does not contain the expected key")
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		if len(userJWT) == 0 {
			fmt.Fprintln(w, "JWT cannot be empty")
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		if _, err := fmt.Fprintf(w, fmt.Sprintf(`{%q: %q}`, apidef.ProTokenKey, res.value)); err != nil {
			fmt.Fprintf(w, "failed to write the response: %v", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusOK)
	}
}
